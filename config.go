package main

import (
	"flag"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"ktkr.us/pkg/manga/core"
)

var cmdConfig = &Command{
	Name:    "config",
	Summary: "[<key> [<value>]]",
	Help: `
Get and set the current series' configuration parameters.`,
	Flags: flag.NewFlagSet("config", flag.ExitOnError),
}

func init() {
	cmdConfig.Run = doConfig
}

func doConfig(cmd *Command, args []string) {
	core.LoadConfig()

	switch len(args) {
	case 0:
		showAllConfig()
	case 1:
		showConfig(args[0])
	case 2:
		setConfig(args[0], args[1])
	default:
		help(cmd)
	}
}

func printConfig(key string, val interface{}) {
	fmt.Printf("%s: %v\n", key, val)
}

func showAllConfig() {
	val := reflect.ValueOf(core.Config)
	t := val.Type()
	for i := 0; i < val.NumField(); i++ {
		ft := t.Field(i)
		if ft.Tag.Get("json") == "-" {
			continue
		}
		fv := val.Field(i)
		printConfig(ft.Name, fv.Interface())
	}
}

func getConfigValue(key string) reflect.Value {
	key = strings.ToLower(key)

	var (
		val = reflect.ValueOf(&core.Config).Elem()
		fv  = val.FieldByNameFunc(func(s string) bool {
			return strings.ToLower(s) == key
		})
	)

	if fv == (reflect.Value{}) {
		cmdConfig.Fatal("no such config parameter: ", key)
	}
	return fv
}

func showConfig(key string) {
	printConfig(key, getConfigValue(key).Interface())
}

func setConfig(key, newval string) {
	val := getConfigValue(key)

	switch val.Type().Kind() {
	case reflect.String:
		val.SetString(newval)
	case reflect.Int:
		i, err := strconv.ParseInt(newval, 10, 64)
		if err != nil {
			cmdConfig.Fatal(err)
		}
		val.SetInt(i)
	}

	showConfig(key)
	core.SaveConfig()
}
