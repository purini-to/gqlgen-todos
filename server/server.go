package main

import (
	"context"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/purini-to/zapmw"
	"github.com/rs/cors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/handler"
	gqlgen_todos "github.com/purini-to/gqlgen-todos"
)

const defaultPort = "8080"
const defaultGracefulTimeout = time.Second * 60

func withRequestID(logger *zap.Logger, r *http.Request) *zap.Logger {
	reqID := middleware.GetReqID(r.Context())
	if len(reqID) == 0 {
		return logger
	}

	return logger.With(zap.String("reqId", reqID))
}

func corsMiddleware() func(http.Handler) http.Handler {
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8080"},
		AllowCredentials: true,
	})

	return c.Handler
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	logger, _ := zap.NewDevelopment()

	r := chi.NewRouter()

	r.Use(
		middleware.RequestID,
		zapmw.WithZap(logger, withRequestID), // logger with request id.
		zapmw.Request(zapcore.InfoLevel, "request"),
		corsMiddleware(),
	)

	r.Handle("/", handler.Playground("GraphQL playground", "/query"))
	r.Handle("/query", handler.GraphQL(gqlgen_todos.NewExecutableSchema(gqlgen_todos.Config{Resolvers: &gqlgen_todos.Resolver{}})))

	s := http.Server{Addr: fmt.Sprintf(":%s", port), Handler: r}

	go func() {
		logger.Info(fmt.Sprintf("Connect to http://localhost:%s/ for GraphQL playground", port))
		logger.Info("Listen and serve", zap.String("transport", "HTTP"), zap.String("port", port))
		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatal("Listen failed", zap.Error(err))
		}
	}()

	sig := make(chan error)
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		sig <- fmt.Errorf("%s", <-c)
	}()

	logger.Info(fmt.Sprintf("SIGNAL %v received, then shutting down...", <-sig), zap.Duration("timeout", defaultGracefulTimeout))
	ctx, cancel := context.WithTimeout(context.Background(), defaultGracefulTimeout)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		// Error from closing listeners, or context timeout:
		logger.Error("Failed to gracefully shutdown", zap.Error(err))
	}

	logger.Info("Exit")
}
