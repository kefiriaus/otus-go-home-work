package main

import "github.com/fixme_my_friend/hw12_13_14_15_calendar/internal/config"

const (
	storageMemory = config.StorageMemory
	storageSQL    = config.StorageSQL
)

type Config = config.Config

func NewConfig() Config                      { return config.New() }
func LoadConfig(path string) (Config, error) { return config.Load(path) }
