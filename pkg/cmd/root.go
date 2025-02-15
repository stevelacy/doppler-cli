/*
Copyright © 2019 Doppler <support@doppler.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/DopplerHQ/cli/pkg/configuration"
	"github.com/DopplerHQ/cli/pkg/controllers"
	"github.com/DopplerHQ/cli/pkg/http"
	"github.com/DopplerHQ/cli/pkg/printer"
	"github.com/DopplerHQ/cli/pkg/utils"
	"github.com/DopplerHQ/cli/pkg/version"
	"github.com/spf13/cobra"
	"gopkg.in/gookit/color.v1"
)

var rootCmd = &cobra.Command{
	Use:   "doppler",
	Short: "The official Doppler CLI",
	Args:  cobra.NoArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		loadFlags(cmd)
		configuration.Setup()
		configuration.LoadConfig()

		if utils.Debug && utils.Silent {
			utils.LogWarning("--silent has no effect when used with --debug")
		}

		// this output does not honor --silent
		printConfig := utils.GetBoolFlagIfChanged(cmd, "print-config", false)
		if printConfig {
			fmt.Println("Active configuration")
			printer.ScopedConfigSource(configuration.LocalConfig(cmd), false, true, true)
			fmt.Println("")
		}

		plain := utils.GetBoolFlagIfChanged(cmd, "plain", false)
		// only run version check if we can print the results
		// --plain doesn't normally affect logging output, but due to legacy reasons it does here
		if utils.CanLogInfo() && !plain {
			checkVersion(cmd.CalledAs())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Usage()
		if err != nil {
			utils.HandleError(err, "Unable to print command usage")
		}
	},
}

func checkVersion(command string) {
	// disable version checking on the "run" command and "secrets download" command
	if command == "run" || command == "download" {
		return
	}

	if !version.PerformVersionCheck || version.IsDevelopment() {
		return
	}

	prevVersionCheck := configuration.VersionCheck()
	// don't check more often than every 24 hours
	if !time.Now().After(prevVersionCheck.CheckedAt.Add(24 * time.Hour)) {
		return
	}

	available, versionCheck, err := controllers.NewVersionAvailable(prevVersionCheck)
	if err != nil {
		// retry on next run
		return
	}

	if !available {
		utils.LogDebug("No CLI updates available")
		// re-use existing version
		versionCheck.LatestVersion = prevVersionCheck.LatestVersion
	} else if utils.IsWindows() {
		utils.Log(fmt.Sprintf("Update: Doppler CLI %s is available\n\nYou can update via 'scoop update doppler'\n", versionCheck.LatestVersion))
	} else {
		utils.Log(color.Green.Sprintf("An update is available."))
		prompt := fmt.Sprintf("Install Doppler CLI %s", versionCheck.LatestVersion)
		if utils.ConfirmationPrompt(prompt, true) {
			installCLIUpdate()
		}
	}

	configuration.SetVersionCheck(versionCheck)
}

func loadFlags(cmd *cobra.Command) {
	var err error
	scope := cmd.Flag("scope").Value.String()
	if configuration.Scope, err = configuration.NormalizeScope(scope); err != nil {
		utils.HandleError(err, fmt.Sprintf("Invalid scope: %s", scope))
	}

	configuration.UserConfigFile = utils.GetPathFlagIfChanged(cmd, "configuration", configuration.UserConfigFile)
	http.TimeoutDuration = utils.GetDurationFlagIfChanged(cmd, "timeout", http.TimeoutDuration)
	http.UseTimeout = !utils.GetBoolFlagIfChanged(cmd, "no-timeout", !http.UseTimeout)
	utils.Debug = utils.GetBoolFlagIfChanged(cmd, "debug", utils.Debug)
	utils.Silent = utils.GetBoolFlagIfChanged(cmd, "silent", utils.Silent)
	// no-file is used by the 'secrets download' command to output secrets to stdout
	utils.Silent = utils.GetBoolFlagIfChanged(cmd, "no-file", utils.Silent)
	utils.OutputJSON = utils.GetBoolFlagIfChanged(cmd, "json", utils.OutputJSON)
	version.PerformVersionCheck = !utils.GetBoolFlagIfChanged(cmd, "no-check-version", !version.PerformVersionCheck)
}

func deprecatedCommand(newCommand string) {
	if newCommand == "" {
		utils.LogWarning("This command is deprecated")
	} else {
		utils.LogWarning(fmt.Sprintf("This command is deprecated, please use the \"doppler %s\" command", newCommand))
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// catch any panics in non-dev builds
	defer func() {
		if !version.IsDevelopment() {
			if err := recover(); err != nil {
				fmt.Fprintf(os.Stderr, fmt.Sprintf("%s %v\n", color.Red.Render("Doppler Exception:"), err))
				os.Exit(1)
			}
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = version.ProgramVersion
	rootCmd.SetVersionTemplate(rootCmd.Version + "\n")
	rootCmd.Flags().BoolP("version", "v", false, "Get the version of the Doppler CLI")

	rootCmd.PersistentFlags().StringP("token", "t", "", "doppler token")
	rootCmd.PersistentFlags().String("api-host", "https://api.doppler.com", "The host address for the Doppler API")
	rootCmd.PersistentFlags().String("dashboard-host", "https://dashboard.doppler.com", "The host address for the Doppler Dashboard")
	rootCmd.PersistentFlags().Bool("no-check-version", !version.PerformVersionCheck, "disable checking for Doppler CLI updates")
	rootCmd.PersistentFlags().Bool("no-verify-tls", false, "do not verify the validity of TLS certificates on HTTP requests (not recommended)")
	rootCmd.PersistentFlags().Bool("no-timeout", !http.UseTimeout, "disable http timeout")
	rootCmd.PersistentFlags().Duration("timeout", http.TimeoutDuration, "max http request duration")

	rootCmd.PersistentFlags().Bool("no-read-env", false, "do not read config from the environment")
	rootCmd.PersistentFlags().String("scope", configuration.Scope, "the directory to scope your config to")
	rootCmd.PersistentFlags().String("configuration", configuration.UserConfigFile, "config file")
	rootCmd.PersistentFlags().Bool("json", utils.OutputJSON, "output json")
	rootCmd.PersistentFlags().Bool("debug", utils.Debug, "output additional information")
	rootCmd.PersistentFlags().Bool("print-config", false, "output active configuration")
	rootCmd.PersistentFlags().Bool("silent", utils.Silent, "disable output of info messages")
}
