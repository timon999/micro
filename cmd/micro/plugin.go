package main

import (
	"errors"
	"io/ioutil"
	"runtime"

	"github.com/layeh/gopher-luar"
	"github.com/yuin/gopher-lua"
)

var loadedPlugins []string

var preInstalledPlugins = []string{
	"go",
	"linter",
}

// Call calls the lua function 'function'
// If it does not exist nothing happens, if there is an error,
// the error is returned
func Call(vm *lua.LState, function string, args []string) error {
	luaFunc := vm.GetGlobal(function)
	if luaFunc.String() == "nil" {
		return errors.New("function does not exist: " + function)
	}
	luaArgs := luar.New(vm, args)
	err := vm.CallByParam(lua.P{
		Fn:      luaFunc,
		NRet:    0,
		Protect: true,
	}, luaArgs)
	return err
}

// LuaFunctionBinding is a function generator which takes the name of a lua function
// and creates a function that will call that lua function
// Specifically it creates a function that can be called as a binding because this is used
// to bind keys to lua functions
func LuaFunctionBinding(function string) func(*View) bool {
	return func(v *View) bool {
		err := CallSync(function, nil)
		if err != nil {
			TermMessage(err)
		}
		return false
	}
}

// LuaFunctionCommand is the same as LuaFunctionBinding except it returns a normal function
// so that a command can be bound to a lua function
func LuaFunctionCommand(function string) func([]string) {
	return func(args []string) {
		err := CallSync(function, args)
		if err != nil {
			TermMessage(err)
		}
	}
}

type LuaFunction struct {
	name string
	args []string
}

var functionChan chan LuaFunction
var functionDone chan error

func CallSync(function string, args []string) error {
	f := LuaFunction{function, args}
	// TermMessage(function)
	functionChan <- f
	// fmt.Printf("\a")
	err := <-functionDone
	RedrawAll()
	return err
}

func CallAsync(function string, args []string) error {
	f := LuaFunction{function, args}
	functionChan <- f
	go func() {
		<-functionDone
		RedrawAll()
	}()
	return nil
}

func InitLua() {
	functionChan = make(chan LuaFunction, 100)
	functionDone = make(chan error, 100)

	go func() {
		luaVM := lua.NewState()
		defer luaVM.Close()

		luaVM.SetGlobal("OS", luar.New(luaVM, runtime.GOOS))
		luaVM.SetGlobal("tabs", luar.New(luaVM, tabs))
		luaVM.SetGlobal("curTab", luar.New(luaVM, curTab))
		luaVM.SetGlobal("messenger", luar.New(luaVM, messenger))
		luaVM.SetGlobal("GetOption", luar.New(luaVM, GetOption))
		luaVM.SetGlobal("AddOption", luar.New(luaVM, AddOption))
		luaVM.SetGlobal("BindKey", luar.New(luaVM, BindKey))
		luaVM.SetGlobal("MakeCommand", luar.New(luaVM, MakeCommand))
		luaVM.SetGlobal("CurView", luar.New(luaVM, CurView))

		LoadPlugins(luaVM)

		for {
			f := <-functionChan
			err := Call(luaVM, f.name, f.args)
			functionDone <- err
		}
	}()
}

// LoadPlugins loads the pre-installed plugins and the plugins located in ~/.config/micro/plugins
func LoadPlugins(luaVM *lua.LState) {
	files, _ := ioutil.ReadDir(configDir + "/plugins")
	for _, plugin := range files {
		if plugin.IsDir() {
			pluginName := plugin.Name()
			files, _ := ioutil.ReadDir(configDir + "/plugins/" + pluginName)
			for _, f := range files {
				if f.Name() == pluginName+".lua" {
					if err := luaVM.DoFile(configDir + "/plugins/" + pluginName + "/" + f.Name()); err != nil {
						TermMessage(err)
						continue
					}
					loadedPlugins = append(loadedPlugins, pluginName)
				}
			}
		}
	}

	for _, pluginName := range preInstalledPlugins {
		plugin := "runtime/plugins/" + pluginName + "/" + pluginName + ".lua"
		data, err := Asset(plugin)
		if err != nil {
			TermMessage("Error loading pre-installed plugin: " + pluginName)
			continue
		}
		if err := luaVM.DoString(string(data)); err != nil {
			TermMessage(err)
			continue
		}
		loadedPlugins = append(loadedPlugins, pluginName)
	}
}
