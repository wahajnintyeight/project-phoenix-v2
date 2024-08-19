package cache

import (
	"context"
	"log"
	"os"
	"project-phoenix/v2/pkg/helper"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	once   sync.Once
	client *redis.Client
}

var redisObj = &Redis{}

func GetInstance() *Redis {
	isRedisDisabled := os.Getenv("DISABLE_REDIS")
	if isRedisDisabled == "true" {
		return nil
	}
	redisObj.once.Do(func() {
		log.Println("Initializing Redis")
		godotenv.Load()
		redisHost := os.Getenv("REDIS_HOST")
		redisPassword := os.Getenv("REDIS_PASSWORD")
		redisUser := os.Getenv("REDIS_USER")
		redisPort := os.Getenv("REDIS_PORT")
		client := redis.NewClient(&redis.Options{
			Addr:     redisHost + ":" + redisPort,
			Password: redisPassword,
			Username: redisUser,
		})
		redisObj.client = client

		pingRes, err := client.Ping(context.Background()).Result()
		if err != nil {
			log.Println("Unable to connect to Redis: ", err)
			return
		} else {
			log.Println("Initialized Redis Connection | Ping Response: ", pingRes)
		}
	})
	return redisObj
}

func (r *Redis) Get(key string) (interface{}, error) {
	if r == nil {
		return true, nil
	}
	ctx := context.Background()
	data, err := r.client.Get(ctx, key).Result()

	if err != nil {
		log.Println("Error fetching from Redis", err)
		return nil, err
	} else {
		return data, nil
	}
}

func (r *Redis) Set(key string, value map[string]interface{}) (bool, error) {
	if r == nil {
		return true, nil
	}
	ctx := context.Background()
	valueByte, marshalEr := helper.MarshalBinary(value)
	if marshalEr != nil {
		log.Println("Error marshalling data", marshalEr)
		return false, marshalEr
	} else {
		err := r.client.Set(ctx, key, valueByte, 0).Err()
		if err != nil {
			log.Println("Error setting in Redis", err)
			return false, err
		} else {
			return true, nil
		}
	}
}

func (r *Redis) SetWithExpiry(key string, value map[string]interface{}, ttlHours int) (bool, error) {
	if r == nil {
		return true, nil
	}
	ctx := context.Background()
	valueByte, marshalEr := helper.MarshalBinary(value)
	if marshalEr != nil {
		log.Println("Error marshalling data", marshalEr)
		return false, marshalEr
	} else {
		ttlPeriod := time.Hour * time.Duration(ttlHours) 
		err := r.client.Set(ctx, key, valueByte, ttlPeriod ).Err()
		if err != nil {
			log.Println("Error setting in Redis", err)
			return false, err
		} else {
			return true, nil
		}
	}
}
