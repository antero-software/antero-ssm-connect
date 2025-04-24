package main

import (
	"bufio"
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/getlantern/systray"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
)

type Profile struct {
	Name       string `mapstructure:"name"`
	Profile    string `mapstructure:"profile"`
	MenuItem   *systray.MenuItem
	InstanceID string `mapstructure:"instanceID"`
}

type Config struct {
	Profiles map[string]Profile `mapstructure:"profiles"`
}

var appConfig Config
var awsPid int

func RunCommand(cmd string) {

}

func StartSSMSession(profile *Profile) {
	// Kill any existing process
	if awsPid != 0 {
		log.Printf("Killing existing process with PID: %d", awsPid)
		cmd := exec.Command("kill", "-9", string(awsPid))
		if err := cmd.Run(); err != nil {
			log.Printf("Error killing process: %s", err)
		}
		awsPid = 0
	}

	// aws ssm start-session --target "Your Instance ID" --document-name AWS-StartPortForwardingSession --parameters "portNumber"=["80"],"localPortNumber"=["56789"]
	log.Printf("Starting session with profile %s", profile.Name)

	cmd := exec.Command("aws", "ssm", "start-session", "--profile", profile.Profile, "--target", profile.InstanceID, "--document-name", "AWS-StartPortForwardingSession", "--parameters", "portNumber=[\"80\"],localPortNumber=[\"56789\"]")

	// Output the command's output to the log
	stdout, err := cmd.StdoutPipe()

	if err != nil {
		log.Printf("Error getting stdout pipe: %s", err)
		return
	}

	err = cmd.Start()

	if err != nil {
		// Show an error message in systray
		systray.SetTooltip("Error starting session: " + err.Error())
		return
	}

	awsPid = cmd.Process.Pid
	log.Printf("Started process with PID: %d", awsPid)

	// Ensure the process is killed when the app exits
	go func() {
		defer cmd.Process.Kill()

		scanner := bufio.NewScanner(stdout)

		for scanner.Scan() {
			log.Println(scanner.Text())
		}

		if err := cmd.Wait(); err != nil {
			log.Printf("Command finished with error: %s", err)
		}
	}()

	systray.SetTooltip("Session started")
}

func profileClicked(profile *Profile) {
	if profile.MenuItem.Checked() {
		profile.MenuItem.Uncheck()
	} else {
		profile.MenuItem.Check()
	}

	// Uncheck all other profiles
	for _, p := range appConfig.Profiles {
		if p.Name != profile.Name {
			p.MenuItem.Uncheck()
		}
	}

	log.Printf("Profile %s clicked, current val %v", profile.Name, profile.MenuItem.Checked())
	StartSSMSession(profile)

	go func() {
		<-profile.MenuItem.ClickedCh
		profileClicked(profile)
	}()
}

func main() {
	// Load all the profiles from the AWS account
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// read the config file
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	if err := viper.Unmarshal(&appConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}

	_, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-southeast-2"))

	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetTitle("SSM Connect")

	for name := range appConfig.Profiles {
		profile := appConfig.Profiles[name] // Create a copy of the map value
		profile.MenuItem = systray.AddMenuItemCheckbox(profile.Name, "", false)
		appConfig.Profiles[name] = profile // Update the map with the modified profile

		go func(profile Profile) {
			<-profile.MenuItem.ClickedCh
			profileClicked(&profile)
		}(profile)
	}

	systray.AddSeparator()

	mQuitOrig := systray.AddMenuItem("Quit", "Quit the whole app")
	go func() {
		<-mQuitOrig.ClickedCh
		systray.Quit()
	}()
}

func onExit() {
	os.Exit(0)
}
