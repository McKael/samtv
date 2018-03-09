// Copyright © 2018 Mikael Berthe <mikael@lilotux.net>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AppName is the name of the CLI application
const AppName = "samtvcli"

var cfgFile string
var server string
var debug bool
var smartDeviceID string
var smartSessionKey string
var smartSessionID int

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   AppName,
	Short: "A CLI remote for Samsung smart TVs",
	Long: `This utility is a command-line interface to send commands to a
Samung "Smart TV" model 2014+.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Define your flags and configuration settings.
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "",
		"config file (default is $HOME/.config/"+AppName+"/"+AppName+".yaml)")
	RootCmd.PersistentFlags().StringVar(&server, "server", "", "TV IP address")
	RootCmd.PersistentFlags().StringVar(&smartDeviceID, "device-uuid", "", "SmartView Device UUID")
	RootCmd.PersistentFlags().StringVar(&smartSessionKey, "session-key", "", "SmartView session key")
	RootCmd.PersistentFlags().IntVar(&smartSessionID, "session-id", -1, "SmartView session ID")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode")

	// Configuration file bindings
	viper.BindPFlag("server", RootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("debug", RootCmd.PersistentFlags().Lookup("debug"))
	viper.BindPFlag("session_key", RootCmd.PersistentFlags().Lookup("session-key"))
	viper.BindPFlag("session_id", RootCmd.PersistentFlags().Lookup("session-id"))
	viper.BindPFlag("device_uuid", RootCmd.PersistentFlags().Lookup("device-uuid"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Skip config file if set to /dev/null
	if cfgFile == "/dev/null" {
		return
	}

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in samtvcli config directory.
		viper.AddConfigPath(home + "/.config/" + AppName)
		viper.SetConfigName(AppName)
	}

	// Read in environment variables that match, with a prefix
	viper.SetEnvPrefix(AppName)
	viper.AutomaticEnv()

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		logrus.Info("Cannot read configuration file: ", err)
	} else {
		debug = viper.GetBool("debug")
		logrus.Info("Using config file: ", viper.ConfigFileUsed())
	}

	// Set up log level
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Get config values from Viper (includes source from
	// configuration file and environment variable)
	server = viper.GetString("server")
	smartDeviceID = viper.GetString("device_uuid")
	smartSessionKey = viper.GetString("session_key")
	smartSessionID = viper.GetInt("session_id")
}
