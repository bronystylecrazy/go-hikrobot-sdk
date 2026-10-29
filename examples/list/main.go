// Command list prints the Hikrobot cameras MVS can see.
package main

import (
	"flag"
	"fmt"
	"log"

	hikrobot "github.com/bronystylecrazy/go-hikrobot-sdk"
)

func main() {
	runtime := flag.String("sdk", "", "path to MvCameraControl.dll (default: the MVS install)")
	flag.Parse()

	sdk, err := hikrobot.Load(*runtime)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Close()
	fmt.Println("MVS runtime", sdk.Version())

	devices, err := sdk.Devices(0)
	if err != nil {
		log.Fatal(err)
	}
	if len(devices) == 0 {
		fmt.Println("no cameras found")
	}
	for i, d := range devices {
		state := "available"
		if !sdk.Accessible(d, hikrobot.AccessExclusive) {
			state = "in use by another program"
		}
		fmt.Printf("%d: %v  vendor=%q name=%q  %s\n", i, d, d.Vendor, d.UserName, state)
	}
}
