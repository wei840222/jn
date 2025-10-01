package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"rogchap.com/v8go"
)

//go:embed third_party/js/*
var jslib embed.FS

type jsLibrary struct {
	name          string
	source        string
	compilerCache *v8go.CompilerCachedData
}

type jsHandler struct {
	isShuttingDown bool
	jslibs         []*jsLibrary
	v8goIsos       chan *v8go.Isolate
}

func (h *jsHandler) loadJSLibrary() error {
	h.jslibs = nil

	v8goIso := v8go.NewIsolate()
	defer v8goIso.Dispose()
	v8goCtx := v8go.NewContext(v8goIso)
	defer v8goCtx.Close()

	return fs.WalkDir(jslib, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".js") {
			b, err := jslib.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			script, err := v8goIso.CompileUnboundScript(string(b), info.Name(), v8go.CompileOptions{})
			if err != nil {
				return err
			}
			script.Run(v8goCtx)
			h.jslibs = append(h.jslibs, &jsLibrary{
				name:          info.Name(),
				source:        string(b),
				compilerCache: script.CreateCodeCache(),
			})
		}
		return nil
	})
}

func (h *jsHandler) generateV8GoContext() (*v8go.Context, func()) {
	v8goIso := <-h.v8goIsos
	v8goCtx := v8go.NewContext(v8goIso)

	for _, jslib := range h.jslibs {
		script, err := v8goIso.CompileUnboundScript(jslib.source, jslib.name, v8go.CompileOptions{CachedData: jslib.compilerCache})
		if err != nil {
			panic(err)
		}
		if _, err := script.Run(v8goCtx); err != nil {
			panic(err)
		}
	}

	return v8goCtx, func() {
		v8goCtx.Close()
		v8goIso.Dispose()
		if !h.isShuttingDown {
			h.v8goIsos <- v8go.NewIsolate()
		}
	}
}

type jsInvokeReq struct {
	Script string `json:"script" binding:"required"`
	Data   any    `json:"data"`
}

type jsInvokeRes struct {
	Result any      `json:"result,omitempty"`
	Logs   []string `json:"logs,omitempty"`
}

type jsInvokeErrRes struct {
	jsInvokeRes
	Error      string `json:"error"`
	Source     string `json:"source,omitempty"`
	StackTrace string `json:"stackTrace,omitempty"`
}

func (h *jsHandler) invoke(c *gin.Context) {
	var req jsInvokeReq
	if strings.Contains(strings.ToLower(string(c.ContentType())), "application/json") {
		if err := c.ShouldBind(&req); err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, &jsInvokeErrRes{
				Error: err.Error(),
			})
			return
		}
	} else {
		req.Data, _ = readMultipartTextOrFile(c, "data")
		script, err := readMultipartTextOrFile(c, "script")
		if err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, &jsInvokeErrRes{
				Error: err.Error(),
			})
			return
		}
		req.Script = script
	}

	v8goCtx, close := h.generateV8GoContext()
	defer close()

	global := v8goCtx.Global()
	if req.Data != nil {
		if s, ok := req.Data.(string); ok {
			if err := global.Set("data", s); err != nil {
				panic(err)
			}
		} else {
			b, err := json.Marshal(req.Data)
			if err != nil {
				panic(err)
			}
			if err := global.Set("data", string(b)); err != nil {
				panic(err)
			}
		}
	}

	if _, err := v8goCtx.RunScript("try { data = JSON.parse(data) } catch {}", "parse.js"); err != nil {
		panic(err)
	}

	var logs []string
	console := v8go.NewObjectTemplate(v8goCtx.Isolate())
	console.Set("log", v8go.NewFunctionTemplate(v8goCtx.Isolate(), func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		var args []string
		for _, arg := range info.Args() {
			switch {
			case arg.IsObject(), arg.IsArray():
				b, _ := json.Marshal(arg.Object())
				args = append(args, string(b))
			case arg.IsString():
				args = append(args, "'"+arg.String()+"'")
			default:
				args = append(args, arg.String())
			}
		}
		logs = append(logs, strings.Join(args, " "))
		return nil
	}))
	consoleO, err := console.NewInstance(v8goCtx)
	if err != nil {
		panic(err)
	}
	if err := global.Set("console", consoleO); err != nil {
		panic(err)
	}

	result, err := v8goCtx.RunScript(req.Script, "script.js")
	if err != nil {
		if jsErr, ok := err.(*v8go.JSError); ok {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, &jsInvokeErrRes{
				Error:      jsErr.Message,
				Source:     jsErr.Location,
				StackTrace: jsErr.StackTrace,
			})
			return
		}
		panic(err)
	}

	if result.IsNullOrUndefined() {
		c.Error(errors.New("the output of script is null or undefined"))
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, &jsInvokeErrRes{
			jsInvokeRes: jsInvokeRes{
				Logs: logs,
			},
			Error:      "The output of script is null or undefined, please make sure that the last line of your script contains the variables to be output.",
			Source:     "",
			StackTrace: "",
		})
		return
	}

	if result.IsObject() {
		c.JSON(http.StatusOK, &jsInvokeRes{
			Result: result.Object(),
			Logs:   logs,
		})
		return
	}

	c.JSON(http.StatusOK, &jsInvokeRes{
		Result: result.String(),
		Logs:   logs,
	})
}

func RegisterJSHandler(lc fx.Lifecycle, e *gin.Engine) error {
	jsConcurrency, _ := strconv.Atoi(os.Getenv("JS_CONCURRENCY"))
	if jsConcurrency <= 0 {
		jsConcurrency = 100
	}

	h := &jsHandler{
		v8goIsos: make(chan *v8go.Isolate, jsConcurrency),
		jslibs:   make([]*jsLibrary, 0),
	}

	if err := h.loadJSLibrary(); err != nil {
		return err
	}

	for i := 0; i < jsConcurrency; i++ {
		h.v8goIsos <- v8go.NewIsolate()
	}

	js := e.Group("/js")
	{
		js.POST("/invoke", h.invoke)
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			h.isShuttingDown = true
			for len(h.v8goIsos) > 0 {
				v8goIso := <-h.v8goIsos
				v8goIso.Dispose()
			}
			return nil
		},
	})

	return nil
}
