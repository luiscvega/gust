package gust

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"log"
	"reflect"

	"github.com/garyburd/redigo/redis"
	"github.com/satori/go.uuid"
)

var (
	saveDigest   string
	deleteDigest string
)

func NewPool() *redis.Pool {
	return &redis.Pool{
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ":6379")
			if err != nil {
				return nil, err
			}

			if saveDigest == "" {
				saveDigest, err = redis.String(c.Do("SCRIPT", "LOAD", saveScript))
				if err != nil {
					log.Fatal(err)
				}
			}

			if deleteDigest == "" {
				deleteDigest, err = redis.String(c.Do("SCRIPT", "LOAD", deleteScript))
				if err != nil {
					log.Fatal(err)
				}
			}

			return c, err
		},
	}
}

// Save struct to hash in redis
func Save(c redis.Conn, src interface{}) error {
	pointer := reflect.ValueOf(src).Elem()

	// Step 1: Get/set id
	modelId := pointer.Field(0).String()
	if modelId == "" {
		modelId = uuid.NewV4().String()
		pointer.Field(0).SetString(modelId)
	}

	// Step 2: Prepare attributes
	var attributesBuffer bytes.Buffer
	encoder := gob.NewEncoder(&attributesBuffer)
	err := encoder.Encode(reflect.ValueOf(src).Interface())
	if err != nil {
		return err
	}

	// Step 3: Prepare indices and uniques
	indices := map[string]interface{}{}
	uniques := map[string]interface{}{}
	for i := 1; i < pointer.NumField(); i++ {
		fieldValue := pointer.Field(i).Interface()
		fieldName := pointer.Type().Field(i).Name
		tagValue := pointer.Type().Field(i).Tag.Get("gust")

		switch tagValue {
		case "":
			continue
		case "index":
			indices[fieldName] = fieldValue
		case "unique":
			uniques[fieldName] = fieldValue
		}
	}

	// Step 4: Prepare indices json
	indicesBytes, err := json.Marshal(indices)
	if err != nil {
		return err
	}

	// Step 5: Prepare uniques json
	uniquesBytes, err := json.Marshal(uniques)
	if err != nil {
		return err
	}

	// Step 6: Get model name
	modelName := pointer.Type().Name()

	// Step 7: Save
	_, err = c.Do("EVALSHA", saveDigest, 0,
		`{ "name": "`+modelName+`", "id": "`+modelId+`" }`,
		attributesBuffer.String(),
		string(indicesBytes),
		string(uniquesBytes))

	return err
}

// FetchOne accepts an id string
func Fetch(c redis.Conn, dst interface{}, id string) error {
	modelName := reflect.ValueOf(dst).Elem().Type().Name()
	key := modelName + ":" + id

	attributesBytes, err := redis.Bytes(c.Do("GET", key))
	if err != nil {
		return err
	}

	readerPointer := bytes.NewReader(attributesBytes)
	decoder := gob.NewDecoder(readerPointer)

	return decoder.Decode(dst)
}

// With returns a record given a unique value
func With(c redis.Conn, dst interface{}, unique string, value string) error {
	modelName := reflect.ValueOf(dst).Elem().Type().Name()
	key := modelName + ":uniques:" + unique

	id, err := redis.String(c.Do("HGET", key, value))
	if err != nil {
		return err
	}

	return Fetch(c, dst, id)
}

// FetchMany accepts a slice of id strings
func FetchMany(c redis.Conn, dst interface{}, ids []string) error {
	slice := reflect.ValueOf(dst).Elem()
	elementType := slice.Type().Elem()
	values := make([]reflect.Value, len(ids))

	for i, id := range ids {
		pointer := reflect.New(elementType).Interface()

		err := Fetch(c, pointer, id)
		if err != nil {
			return err
		}

		values[i] = reflect.Indirect(reflect.ValueOf(pointer))
	}

	slice.Set(reflect.Append(slice, values...))

	return nil
}

// FetchAll gets all records of the model
func FetchAll(c redis.Conn, dst interface{}) error {
	modelName := reflect.TypeOf(dst).Elem().Elem().Name()

	ids, err := redis.Strings(c.Do("SMEMBERS", modelName+":all"))
	if err != nil {
		return err
	}

	return FetchMany(c, dst, ids)
}

// Find fetches records that match the given index
func Find(c redis.Conn, dst interface{}, queries ...string) error {
	modelName := reflect.TypeOf(dst).Elem().Elem().Name()

	args := []interface{}{}
	for _, query := range queries {
		args = append(args, modelName+":indices:"+query)
	}

	ids, err := redis.Strings(c.Do("SINTER", args...))
	if err != nil {
		return err
	}

	return FetchMany(c, dst, ids)
}

func Delete(c redis.Conn, modelName, modelId string) (bool, error) {
	return redis.Bool(c.Do("EVALSHA", deleteDigest, 0,
		`{ "name": "`+modelName+`", "id": "`+modelId+`" }`))
}
