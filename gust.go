package gust

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/garyburd/redigo/redis"
	"github.com/satori/go.uuid"
)

type Conn struct {
	redis.Conn
}

func NewConn(url string) (Conn, error) {
	conn := Conn{}

	c, err := redis.DialURL(url)
	if err != nil {
		return conn, err
	}

	conn.Conn = c

	return conn, nil
}

// Save struct to hash in redis
func (c Conn) Save(model interface{}) error {
	elem := reflect.ValueOf(model).Elem()
	id := elem.Field(0).String()

	if id == "" {
		id = uuid.NewV4().String()
		elem.Field(0).SetString(id)
	}

	// Step 1: Prepare attributes
	var attributesBuffer bytes.Buffer
	encoder := gob.NewEncoder(&attributesBuffer)
	err := encoder.Encode(reflect.ValueOf(model).Interface())
	if err != nil {
		return err
	}

	// Step 3: Prepare indices and uniques
	indices := map[string]string{}
	uniques := map[string]string{}
	for i := 1; i < elem.NumField(); i++ {
		fieldValue := fmt.Sprint(elem.Field(i).Interface())
		fieldName := elem.Type().Field(i).Name
		tagValue := elem.Type().Field(i).Tag.Get("gust")

		if fieldValue == "" {
			continue
		}

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
	modelName := elem.Type().Name()

	// Step 7: Save
	_, err = c.Do("EVAL", saveScript, 0,
		`{ "name": "`+modelName+`", "id": "`+id+`" }`,
		attributesBuffer.String(),
		string(indicesBytes),
		string(uniquesBytes))

	return err
}

// Fetch accepts an id string
func (c Conn) Fetch(model interface{}, id string) error {
	modelName := reflect.ValueOf(model).Elem().Type().Name()
	key := modelName + ":" + id

	attributesBytes, err := redis.Bytes(c.Do("GET", key))
	if err != nil {
		return err
	}

	readerPointer := bytes.NewReader(attributesBytes)
	decoder := gob.NewDecoder(readerPointer)

	return decoder.Decode(model)
}

// FetchMany accepts a slice of id strings
func (c Conn) FetchMany(model interface{}, ids []string) error {
	slice := reflect.ValueOf(model).Elem()
	elementType := slice.Type().Elem()
	values := make([]reflect.Value, len(ids))

	for i, id := range ids {
		pointer := reflect.New(elementType).Interface()

		err := c.Fetch(pointer, id)
		if err != nil {
			return err
		}

		values[i] = reflect.Indirect(reflect.ValueOf(pointer))
	}

	slice.Set(reflect.Append(slice, values...))

	return nil
}

// FetchAll gets all records of the model
func (c Conn) FetchAll(model interface{}) error {
	modelName := reflect.TypeOf(model).Elem().Elem().Name()

	ids, err := redis.Strings(c.Do("SMEMBERS", modelName+":all"))
	if err != nil {
		return err
	}

	return c.FetchMany(model, ids)
}

// With returns a record given a unique value
func (c Conn) With(model interface{}, unique string, value string) error {
	modelName := reflect.ValueOf(model).Elem().Type().Name()
	key := modelName + ":uniques:" + unique

	id, err := redis.String(c.Do("HGET", key, value))
	if err != nil {
		return err
	}

	return c.Fetch(model, id)
}

// Find fetches records that match the given index
func (c Conn) Find(model interface{}, queries ...string) error {
	modelName := reflect.TypeOf(model).Elem().Elem().Name()

	args := []interface{}{}
	for _, query := range queries {
		args = append(args, modelName+":indices:"+query)
	}

	ids, err := redis.Strings(c.Do("SINTER", args...))
	if err != nil {
		return err
	}

	return c.FetchMany(model, ids)
}

func (c Conn) Delete(modelName, modelId string) (bool, error) {
	return redis.Bool(c.Do("EVAL", deleteScript, 0,
		`{ "name": "`+modelName+`", "id": "`+modelId+`" }`))
}
