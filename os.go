package main

import (
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"
)

const assetPrefix string = "/$asset$/"

func InitOS() {
	Sub("os", func(buf []byte) []byte {
		msg := &Msg{}
		check(proto.Unmarshal(buf, msg))
		switch msg.Command {
		case Msg_CODE_FETCH:
			return HandleCodeFetch(
				msg.CodeFetchModuleSpecifier,
				msg.CodeFetchContainingFile)
		case Msg_CODE_CACHE:
			return HandleCodeCache(
				msg.CodeCacheFilename,
				msg.CodeCacheSourceCode,
				msg.CodeCacheOutputCode)
		case Msg_EXIT:
			os.Exit(int(msg.ExitCode))
		default:
			panic("[os] Unexpected message " + string(buf))
		}
		return nil
	})
}

func ResolveModule(moduleSpecifier string, containingFile string) (
	moduleName string, filename string, err error) {

	logDebug("ResolveModule %s %s", moduleSpecifier, containingFile)

	moduleUrl, err := url.Parse(moduleSpecifier)
	if err != nil {
		return
	}
	baseUrl, err := url.Parse(containingFile)
	if err != nil {
		return
	}
	resolved := baseUrl.ResolveReference(moduleUrl)
	moduleName = resolved.String()
	if moduleUrl.IsAbs() {
		filename = path.Join(SrcDir, resolved.Host, resolved.Path)
	} else {
		filename = resolved.Path
	}
	return
}

func HandleCodeFetch(moduleSpecifier string, containingFile string) (out []byte) {
	assert(moduleSpecifier != "", "moduleSpecifier shouldn't be empty")
	res := &Msg{}
	var sourceCodeBuf []byte
	var err error

	defer func() {
		if err != nil {
			res.Error = err.Error()
		}
		out, err = proto.Marshal(res)
		check(err)
	}()

	moduleName, filename, err := ResolveModule(moduleSpecifier, containingFile)
	if err != nil {
		return
	}

	logDebug("HandleCodeFetch moduleSpecifier %s containingFile %s filename %s",
		moduleSpecifier, containingFile, filename)

	if isRemote(moduleName) {
		sourceCodeBuf, err = FetchRemoteSource(moduleName, filename)
	} else if strings.HasPrefix(moduleName, assetPrefix) {
		f := strings.TrimPrefix(moduleName, assetPrefix)
		sourceCodeBuf, err = Asset("dist/" + f)
		if err != nil {
			logDebug("%s Asset doesn't exist. Return without error", moduleName)
			err = nil
		}
	} else {
		assert(moduleName == filename,
			"if a module isn't remote, it should have the same filename")
		sourceCodeBuf, err = ioutil.ReadFile(moduleName)
	}
	if err != nil {
		return
	}

	outputCode, err := LoadOutputCodeCache(filename, sourceCodeBuf)
	if err != nil {
		return
	}

	var sourceCode = string(sourceCodeBuf)
	var command = Msg_CODE_FETCH_RES
	res = &Msg{
		Command:                command,
		CodeFetchResModuleName: moduleName,
		CodeFetchResFilename:   filename,
		CodeFetchResSourceCode: sourceCode,
		CodeFetchResOutputCode: outputCode,
	}
	return
}

func HandleCodeCache(filename string, sourceCode string,
	outputCode string) []byte {

	fn := CacheFileName(filename, []byte(sourceCode))
	outputCodeBuf := []byte(outputCode)
	err := ioutil.WriteFile(fn, outputCodeBuf, 0600)
	res := &Msg{}
	if err != nil {
		res.Error = err.Error()
	}
	out, err := proto.Marshal(res)
	check(err)
	return out
}
