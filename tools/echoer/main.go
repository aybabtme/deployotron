//!build
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	sleep := flag.Duration("sleep", time.Second, "how long to sleep between echoes")
	repeat := flag.Bool("repeat", true, "whether to echo forever")
	flag.Parse()

	for {
		time.Sleep(*sleep)

		fmt.Printf(
			"%d(%d): %s\n",
			os.Getpid(),
			os.Getppid(),
			strings.Join(flag.Args(), " "),
		)
		if !*repeat {
			return
		}
	}
}
