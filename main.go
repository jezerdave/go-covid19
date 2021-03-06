package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/jezerdave/go-covid19/covid"
	"github.com/jezerdave/go-covid19/covid/util/jsons"
	_ "github.com/jezerdave/go-covid19/docs"
	"github.com/jezerdave/go-covid19/src/http/rest"
	"github.com/jezerdave/go-covid19/src/storage"
	"github.com/jezerdave/go-covid19/src/updating"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/swaggo/echo-swagger"
	"log"
	"os"
	"os/signal"
	"time"
)

var (
	port      = "1919"
	redisHost = "127.0.0.1"
	redisPass = ""
	redisPort = "6379"
)

// @title GO-COVID19 API
// @version 1.0
// @description REST Api for covid-19 cases
// @BasePath /api/v1
// @schemes https
// @host go-covid19.sideprojects.fun
// @contact.name Jezer Dave Bacquian
// @contact.url https://github.com/jezerdave
// @contact.email jezerdavebacquian@gmail.com
func main() {

	getConfig()
	kV, err := initRedis(fmt.Sprintf("%s:%s", redisHost, redisPort), redisPass)
	if err != nil {
		log.Fatal(err)
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	srv := covid.NewClient()
	repo := storage.NewStorage(kV)
	countries, _ := jsons.JsonCountries()
	states, _ := jsons.JsonStates()

	updSrv := updating.NewService(repo, *srv, *countries, *states)

	routes := rest.Handler{
		R:     repo,
		UpSrv: updSrv,
	}

	v := e.Group("api/v1")

	cG := v.Group("/countries")
	cG.GET("", routes.GetCountriesData)
	cG.GET("/", routes.GetCountriesData)
	cG.GET("/:country", routes.FindCountry)

	sG := v.Group("/states")
	sG.GET("", routes.GetStatesData)
	sG.GET("/", routes.GetStatesData)
	sG.GET("/:state", routes.FindStates)

	phDoh := v.Group("/doh/ph")
	phDoh.GET("", routes.GetPHStats)
	phDoh.GET("eys", routes.GetPHStats)
	phDoh.GET("/hospital-pui", routes.GetPHHospitalPUIs)

	hG := v.Group("/histories")
	hG.GET("", routes.GetHistories)
	hG.GET("/", routes.GetHistories)
	hG.GET("/:country", routes.FindCountryHistories)

	v.GET("/update", routes.UpdateData)
	v.GET("/docs/*", echoSwagger.WrapHandler)

	uptimeTicker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-uptimeTicker.C:
				fmt.Println("UPDATING DATA")
				go updSrv.UpdateData()
			}
		}
	}()

	go func() {
		if err := e.Start(fmt.Sprintf(":%s", port)); err != nil {
			e.Logger.Info("shutting down the server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	kV.Close()
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func initRedis(addr string, pw string) (*redis.Client, error) {

	fmt.Printf("[REDIS] Connecting at %s \n", addr)

	kV := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: pw,
		DB:       0,
	})

	for {
		_, err := kV.Ping().Result()
		if err != nil {
			return nil, err
		}
		break
	}

	log.Printf("[REDIS] Redis Client Connected")
	return kV, nil
}

func getConfig() {
	if os.Getenv("APP_PORT") != "" {
		port = os.Getenv("APP_PORT")
	}
	if os.Getenv("REDIS_HOST") != "" {
		redisHost = os.Getenv("REDIS_HOST")
	}
	if os.Getenv("REDIS_PASS") != "" {
		redisPass = os.Getenv("REDIS_PASS")
	}
	if os.Getenv("REDIS_PORT") != "" {
		redisPort = os.Getenv("REDIS_PORT")
	}
}
