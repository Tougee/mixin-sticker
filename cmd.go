package main

import (
	"log"
	"os/exec"
)

func callSpider(cmdStr string) error {
	log.Printf("run cmd: %v", cmdStr)
	cmd := exec.Command("bash", "-c", cmdStr)
	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start cmd: %v", err.Error())
		return err
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("Cmd returned error: %v", err.Error())
		return err
	}
	return nil
}
