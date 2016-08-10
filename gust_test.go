package gust

import "testing"

type Address struct {
	Line1 string
}

type User struct {
	Id      string
	Age     int `gust:"unique"`
	Name    string
	Email   string `gust:"unique"`
	Sex     string `gust:"index"`
	College string `gust:"index"`
	Address Address
}

func TestA(t *testing.T) {
	c, err := NewConn("redis://localhost:6379")
	if err != nil {
		t.Error(err)
	}
	defer c.Close()

	// Create user
	user := User{"", 28, "Luis", "luis@vega.com", "", "Ateneo de Manila", Address{Line1: "79"}}
	err = c.Save(&user)
	if err != nil {
		t.Error(err)
	}

	id := user.Id

	// Fetch user
	user = User{}
	err = c.Fetch(&user, id)
	if err != nil {
		t.Error(err)
	}

	// Find users
	users := []User{}
	err = c.FetchAll(&users)
	if err != nil {
		t.Error(err)
	}

	// Delete user
	ok, err := c.Delete("User", id)
	if err != nil {
		t.Error(err)
	}

	if !ok {
		t.Error("Failed to delete")
	}
}
