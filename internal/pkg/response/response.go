package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Body adalah format respons JSON seragam untuk semua endpoint.
type Body struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// OK mengirim respons sukses (200).
func OK(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, Body{
		Success: true,
		Message: "OK",
		Data:    data,
	})
}

// Created mengirim respons sukses pembuatan data (201).
func Created(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusCreated, Body{
		Success: true,
		Message: "Created",
		Data:    data,
	})
}

// Error mengirim respons gagal dengan status HTTP tertentu.
func Error(c echo.Context, status int, message string) error {
	return c.JSON(status, Body{
		Success: false,
		Message: message,
	})
}
