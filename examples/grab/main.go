// Command grab captures frames from a Hikrobot camera and saves the last one
// as a PNG.
package main

import (
	"flag"
	"image/png"
	"log"
	"os"
	"time"

	hikrobot "github.com/bronystylecrazy/go-hikrobot-sdk"
)

func main() {
	var (
		runtime  = flag.String("sdk", "", "path to MvCameraControl.dll (default: the MVS install)")
		camera   = flag.String("camera", "", "serial, IP, model or user name (default: first camera)")
		count    = flag.Int("n", 10, "frames to grab")
		exposure = flag.Float64("exposure", 0, "exposure time in µs (0 keeps the camera's setting)")
		trigger  = flag.Bool("trigger", false, "use software triggers instead of free-running acquisition")
		out      = flag.String("out", "frame.png", "where to save the last frame")
	)
	flag.Parse()

	sdk, err := hikrobot.Load(*runtime)
	if err != nil {
		log.Fatal(err)
	}
	defer sdk.Close()

	var device hikrobot.Device
	if *camera != "" {
		device, err = sdk.Find(*camera)
	} else {
		var devices []hikrobot.Device
		if devices, err = sdk.Devices(0); err == nil && len(devices) == 0 {
			log.Fatal("no cameras found")
		} else if err == nil {
			device = devices[0]
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	cam, err := sdk.Open(device)
	if err != nil {
		log.Fatalf("open %v: %v", device, err)
	}
	defer cam.Close()
	_ = cam.OnException(func(e hikrobot.Exception) { log.Printf("camera event: %v", e) })

	if *trigger {
		must(cam.SetEnumString("TriggerMode", "On"))
		must(cam.SetEnumString("TriggerSource", "Software"))
	} else {
		must(cam.SetEnumString("TriggerMode", "Off"))
	}
	if *exposure > 0 {
		must(cam.SetEnumString("ExposureAuto", "Off"))
		must(cam.SetFloat("ExposureTime", float32(*exposure)))
	}
	format, _ := cam.GetEnumString("PixelFormat")
	log.Printf("opened %v, pixel format %s", device, format)

	must(cam.StartGrabbing())
	var last *hikrobot.Frame
	start := time.Now()
	for i := 0; i < *count; i++ {
		if *trigger {
			must(cam.Execute("TriggerSoftware"))
		}
		f, err := cam.Grab(3 * time.Second)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("frame %d: %dx%d %v, lost packets %d", f.Number, f.Width, f.Height, f.PixelType, f.LostPackets)
		if last != nil {
			last.Release()
		}
		last = f
	}
	if last == nil {
		return
	}
	defer last.Release()
	log.Printf("%.1f fps", float64(*count)/time.Since(start).Seconds())

	img, err := cam.Image(last)
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	must(png.Encode(file, img))
	log.Printf("saved %s", *out)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
