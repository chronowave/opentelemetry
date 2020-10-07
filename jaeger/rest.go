package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/chronowave/chronowave/embed"
	"github.com/labstack/echo/v4"
)

func startEcho(stream *embed.WaveStream, port int) *echo.Echo {
	e := echo.New()
	e.Logger.SetOutput(ioutil.Discard)

	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, http.StatusText(http.StatusOK))
	})

	e.GET("/query", func(c echo.Context) error {
		var (
			data []byte
			err  error
		)
		defer func() {
			if err := recover(); err != nil {
				logger.Error("got error %v while processing query %s", err, string(data))
			}
		}()
		req := c.Request()
		data, err = ioutil.ReadAll(req.Body)
		if err != nil {
			return err
		}
		logger.Warn("processing query: %s", string(data))
		data, err = stream.Query(req.Context(), string(data))
		if err != nil {
			return err
		}

		return c.Stream(http.StatusOK, echo.MIMEApplicationJSON, bytes.NewReader(data))
	})
	go func() {
		err := e.Start(":" + strconv.FormatInt(int64(port), 10))
		logger.Error("http listener error: %v", err)
	}()
	return e
}
