package main

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
)

func (app *Config) routes() http.Handler {
	mux := chi.NewRouter()

	mux.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	mux.Use(middleware.Heartbeat("/ping"))
	mux.Use(app.requestContextMiddleware())
	mux.Use(app.httpAccessLogMiddleware())
	mux.Use(app.prometheusMiddleware())
	mux.Use(app.rateLimitMiddleware())

	if app.metricsHandler != nil && app.metricsPath != "" {
		mux.Handle(app.metricsPath, app.metricsHandler)
	}

	mux.Get("/health", app.Health)
	mux.Post("/auth/register", app.Register)
	mux.Post("/auth/login", app.Login)
	mux.Get("/users/{userID}", app.GetUser)
	mux.Get("/users/{userID}/exists", app.UserExists)

	mux.Group(func(r chi.Router) {
		r.Use(app.authMiddleware())
		r.Get("/history/{userID}", app.GetHistory)
		r.Post("/topups", app.TopUp)
		r.Post("/transfers", app.CreateTransfer)
	})

	return mux
}
