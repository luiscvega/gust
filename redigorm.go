package redigorm

import (
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strconv"

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

func Attributes(pointer reflect.Value) (map[string]interface{}, error) {
	attributesMap := map[string]interface{}{}

	for i := 0; i < pointer.NumField(); i++ {
		fieldValue := pointer.Field(i)
		fieldName := pointer.Type().Field(i).Tag.Get("redis")

		if fieldName == "" {
			continue
		}

		if fieldValue.Type().Kind() == reflect.Struct {
			fv, err := Attributes(reflect.Indirect(fieldValue))
			if err != nil {
				return nil, err
			}

			attributesBytes, err := json.Marshal(fv)
			if err != nil {
				return nil, err
			}

			attributesMap[fieldName] = string(attributesBytes)
			continue
		}

		attributesMap[fieldName] = fieldValue.Interface()
	}

	return attributesMap, nil
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
	attributesMap, err := Attributes(pointer)
	if err != nil {
		return err
	}

	attributes := []interface{}{}

	for k, v := range attributesMap {
		attributes = append(attributes, k, v)
	}

	// Step 3: Prepare indices and uniques
	indices := map[string][]interface{}{}
	uniques := map[string]interface{}{}
	for i := 0; i < pointer.NumField(); i++ {
		fieldValue := pointer.Field(i).Interface()
		fieldName := pointer.Type().Field(i).Tag.Get("redis")
		tagValue := pointer.Type().Field(i).Tag.Get("omg")

		if fieldName == "" {
			continue
		}

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

	// Step 4: Prepare attributes json
	attributesBytes, err := json.Marshal(attributes)
	if err != nil {
		return err
	}

	// Step 5: Prepare indices json
	indicesBytes, err := json.Marshal(indices)
	if err != nil {
		return err
	}

	// Step 6: Prepare uniques json
	uniquesBytes, err := json.Marshal(uniques)
	if err != nil {
		return err
	}

	// Step 7: Get model name
	modelName := pointer.Type().Name()

	// Step 8: Save
	_, err = c.Do("EVALSHA", saveDigest, 0,
		`{ "name": "`+modelName+`", "id": "`+modelId+`" }`,
		string(attributesBytes),
		string(indicesBytes),
		string(uniquesBytes))

	return err
}

// FetchOne accepts an id string
func Fetch(c redis.Conn, dst interface{}, id string) error {
	modelName := reflect.ValueOf(dst).Elem().Type().Name()
	key := modelName + ":" + id

	values, err := redis.Values(c.Do("HGETALL", key))
	if err != nil {
		return err
	}

	attributesMap := map[string]interface{}{}
	for i := 0; i < len(values); i += 2 {
		attributesMap[string(values[i].([]byte))] = values[i+1]
	}

	pointer := reflect.ValueOf(dst).Elem()
	for i := 1; i < pointer.NumField(); i++ {
		field := pointer.Field(i)
		structField := pointer.Type().Field(i)
		fieldName := structField.Tag.Get("redis")

		if fieldName == "" {
			continue
		}

		value := attributesMap[fieldName]
		switch structField.Type.Kind() {
		case reflect.String:
			field.SetString(string(value.([]byte)))
		case reflect.Int:
			intString := string(value.([]byte))

			integer, err := strconv.Atoi(intString)
			if err != nil {
				return err
			}

			field.Set(reflect.ValueOf(integer))
		case reflect.Struct:
			x := reflect.New(field.Type())

			fmt.Println(string(value.([]byte)))
			err := json.Unmarshal(value.([]byte), x.Interface())
			if err != nil {
				return err
			}

			fmt.Println("=>", x)
		}
	}

	return nil
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
