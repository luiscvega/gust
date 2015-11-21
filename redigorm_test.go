package redigorm

import "testing"

type Address struct {
	Line1 string
}

type User struct {
	Id      string
	Age     int
	Name    string
	Email   string `omg:"unique"`
	Sex     string `omg:"index"`
	College string `omg:"index"`
	Address Address
}

func TestA(t *testing.T) {
	c := Open()
	defer c.Close()

	c.Do("FLUSHALL")

	Save(c, &User{"", 28, "Luis", "luis@vega.com", "male", "Ateneo", Address{Line1: "79"}})
	Save(c, &User{"", 33, "Miguel", "miguel@vega.com", "male", "UP", Address{Line1: "79"}})
	Save(c, &User{"", 31, "Paola", "paola@vega.com", "female", "Ateneo", Address{Line1: "79"}})

	user := User{}
	err := With(c, &user, "Email", "luis@vega.com")
	if err != nil {
		t.Error(err)
	}

	users := []User{}
	err = Find(c, &users, "Sex:male")
	if err != nil {
		t.Error(err)
	}

	users = []User{}
	err = FetchAll(c, &users)
	if err != nil {
		t.Error(err)
	}
}
