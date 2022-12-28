package main

import (
	"github.com/spf13/viper"
	"testing"
)

func TestConfig(t *testing.T) {
	cfg := &Config{}
	err := viper.ReadInConfig()
	if err != nil {
		t.Error(err)
	}
	err = viper.Unmarshal(cfg)
	if err != nil {
		t.Error(err)
	}
	t.Logf("parsed config: %+v", *cfg)
}
