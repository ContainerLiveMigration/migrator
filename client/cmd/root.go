package cmd

import (
	"cr/migrator"
	"log"
	"net/rpc"
	"os"
	"os/user"

	"github.com/spf13/cobra"
)

const (
	localhost = "127.0.0.1"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "migrate",
	Short: "migrate an existing container to a new host",
	Long:  `migrate an existing container to a new host`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		// get username of current user
		user, err := user.Current()
		if err != nil {
			log.Printf("get current user failed: %v", err)
			os.Exit(1)
		}
		log.Printf("current user: %s", user.Username)
		instanceName := args[0]
		targetIP := args[1]
		diskless, err := cmd.Flags().GetBool("diskless")
		if err != nil {
			log.Printf("get diskless flag failed: %v", err)
			os.Exit(1)
		}
		log.Printf("migrating instance %s to %s", instanceName, targetIP)
		client, err := rpc.DialHTTP("tcp", localhost+migrator.RPCPort)
		if err != nil {
			log.Printf("dial http failed: %v", err)
			os.Exit(1)
		}
		if diskless {
			r := migrator.DisklessMigrateResponse{}
			err = client.Call("Migrator.DisklessMigrate", &migrator.DisklessMigrateRequest{
				UserName:     user.Username,
				InstanceName: instanceName,
				Target:       targetIP,
			}, &r)
			if err != nil || r.Status != migrator.OK {
				log.Printf("diskless migrate failed: %v", err)
				os.Exit(1)
			}
		} else {
			r := migrator.MigrateResponse{}
			err = client.Call("Migrator.Migrate", &migrator.MigrateRequest{
				UserName:     user.Username,
				InstanceName: instanceName,
				Target:       targetIP,
			}, &r)
			if err != nil || r.Status != migrator.OK {
				log.Printf("migrate failed: %v", err)
				os.Exit(1)
			}
		}
		log.Printf("migrate success")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cr.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().BoolP("diskless", "d", false, "diskless migration")
}
