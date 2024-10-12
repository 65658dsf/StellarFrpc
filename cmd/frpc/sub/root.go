// Copyright 2018 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sub

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"net/http"
	"bytes"

	"github.com/spf13/cobra"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/util/log"
	"github.com/fatedier/frp/pkg/util/version"
)

var (
	cfgFile          string
	cfgDir           string
	showVersion      bool
	strictConfigMode bool
	token            string
	tunnels          []string
)

const api = "https://api.stellarfrp.top"

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./frpc.ini", "需要被启动的隧道的配置文件。")
	rootCmd.PersistentFlags().StringVarP(&cfgDir, "config_dir", "", "", "需要被启动的隧道的配置文件目录。")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "输出版本号。")
	rootCmd.PersistentFlags().BoolVarP(&strictConfigMode, "strict_config", "", true, "严格配置解析模式，未知字段将导致错误。")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "u", "", "从StellarConsole获取的Token。")
	rootCmd.PersistentFlags().StringSliceVarP(&tunnels, "tunnel", "t", []string{}, "需要被启动的隧道名，多个隧道以英文逗号分隔。")
}

var rootCmd = &cobra.Command{
	Use:   "StellarFrpc",
	Short: "frpc is the client of frp (https://github.com/fatedier/frp)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Full())
			return nil
		}

		if token != "" && len(tunnels) != 0 {
			log.Infof("正在获取隧道配置...")
			data := getUserTunnels()
			for _, tunnel := range tunnels {
				if v, ok := data[tunnel].(map[string]interface{}); ok {
					content := v["data"].(string)
					runClient(content, true)
				} else {
					log.Warnf("此隧道不存在: %s", tunnel)
				}
			}
		} else {
			// If cfgDir is not empty, run multiple frpc service for each config file in cfgDir.
			// Note that it's only designed for testing. It's not guaranteed to be stable.
			if cfgDir != "" {
				_ = runMultipleClients(cfgDir, false)
				return nil
			}
	
			// Do not show command usage here.
			err := runClient(cfgFile, false)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
		return nil
	},
}

func runMultipleClients(cfgDir string, alreadyRead bool) error {
	var wg sync.WaitGroup
	err := filepath.WalkDir(cfgDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		wg.Add(1)
		time.Sleep(time.Millisecond)
		go func() {
			defer wg.Done()
			err := runClient(path, alreadyRead)
			if err != nil {
				fmt.Printf("frpc service error for config file [%s]\n", path)
			}
		}()
		return nil
	})
	wg.Wait()
	return err
}

func Execute() {
	rootCmd.SetGlobalNormalizationFunc(config.WordSepNormalizeFunc)
	log.InitLogger("console", "info", 3, false)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func handleTermSignal(svr *client.Service) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	svr.GracefulClose(500 * time.Millisecond)
}

func runClient(cfgFilePath string, alreadyRead bool) error {
	cfg, proxyCfgs, visitorCfgs, isLegacyFormat, err := config.LoadClientConfig(cfgFilePath, strictConfigMode, alreadyRead)
	if err != nil {
		return err
	}
	if isLegacyFormat {
		fmt.Printf("ini 格式已不再推荐使用，将在以后的版本中移除支持，请改用 yaml/json/toml 格式！")
	}

	warning, err := validation.ValidateAllClientConfig(cfg, proxyCfgs, visitorCfgs)
	if warning != nil {
		fmt.Printf("警告: %v\n", warning)
	}
	if err != nil {
		return err
	}
	return startService(cfg, proxyCfgs, visitorCfgs, cfgFilePath)
}

func startService(
	cfg *v1.ClientCommonConfig,
	proxyCfgs []v1.ProxyConfigurer,
	visitorCfgs []v1.VisitorConfigurer,
	cfgFile string,
) error {
	log.InitLogger(cfg.Log.To, cfg.Log.Level, int(cfg.Log.MaxDays), cfg.Log.DisablePrintColor)

	svr, err := client.NewService(client.ServiceOptions{
		Common:         cfg,
		ProxyCfgs:      proxyCfgs,
		VisitorCfgs:    visitorCfgs,
		ConfigFilePath: cfgFile,
	})
	if err != nil {
		return err
	}

	shouldGracefulClose := cfg.Transport.Protocol == "kcp" || cfg.Transport.Protocol == "quic"
	// Capture the exit signal if we use kcp or quic.
	if shouldGracefulClose {
		go handleTermSignal(svr)
	}
	return svr.Run(context.Background())
}

func getUserTunnels() map[string]interface{} {
	payload := map[string]string{
		"token": token,
	}
	jsonValue, _ := json.Marshal(payload)
	resp, err := http.Post(api+"/GetUserTunnel", "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Errorf("获取隧道信息失败: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Errorf("获取隧道信息失败")
		os.Exit(1)
	}

	var userTunnels	map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&userTunnels)

	var data map[string]interface{} = userTunnels["tunnel"].(map[string]interface{})

	return data
}