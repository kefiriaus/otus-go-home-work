package main

import (
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/config"
	"github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/worker"
)

func main() { worker.Main(config.Sender, "configs/sender_config.yaml") }
