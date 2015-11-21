package redigorm

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"log"
	"reflect"

	"github.com/garyburd/redigo/redis"
	"github.com/satori/go.uuid"
)

var saveDigest string

func Open() redis.Conn {
	c, err := redis.Dial("tcp", ":6379")
	if err != nil {
		log.Fatal(err)
	}

	saveDigest, err = redis.String(c.Do("SCRIPT", "LOAD", saveScript))
	if err != nil {
		log.Fatal(err)
	}

	return c
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
	indices := map[string][]interface{}{}
	uniques := map[string]interface{}{}
	for i := 1; i < pointer.NumField(); i++ {
		fieldValue := pointer.Field(i).Interface()
		fieldName := pointer.Type().Field(i).Name
		tagValue := pointer.Type().Field(i).Tag.Get("omg")

		if tagValue == "" {
			continue
		}

		if tagValue == "index" {
			s, ok := indices[fieldName]
			if ok {
				s = append(s, fieldValue)
				continue
			}

			indices[fieldName] = []interface{}{fieldValue}
			continue
		}

		if tagValue == "unique" {
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

	attributesBufferPointer := bytes.NewReader(attributesBytes)
	decoder := gob.NewDecoder(attributesBufferPointer)

	return decoder.Decode(dst)
}

// FetchMany accepts a slice of id strings
func FetchMany(c redis.Conn, dst interface{}, ids []string) error {
	slice := reflect.ValueOf(dst).Elem()
	elementType := slice.Type().Elem()
	values := []reflect.Value{}

	for _, id := range ids {
		pointer := reflect.New(elementType).Interface()

		err := Fetch(c, pointer, id)
		if err != nil {
			return err
		}

		values = append(values, reflect.Indirect(reflect.ValueOf(pointer)))
	}

	slice.Set(reflect.Append(slice, values...))

	return nil
}

// FetchAll gets all records of the model
func FetchAll(c redis.Conn, dst interface{}) error {
	modelName := reflect.TypeOf(dst).Elem().Elem().Name()

	ids, err := All(c, modelName)
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

// All returns all ids of a model
func All(c redis.Conn, modelName string) ([]string, error) {
	return redis.Strings(c.Do("SMEMBERS", modelName+":all"))
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
