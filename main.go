package main

import (
        "os"
        "github.com/jeremyweader/cmkbeat/cmd"
)

func main() {
//	beat.Run("cmkbeat", "", beater.New)
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
