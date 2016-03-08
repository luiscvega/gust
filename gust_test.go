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
	pool, err := NewPool("localhost:6379")
	if err != nil {
		t.Error(err)
	}
	defer pool.Close()

	// Save user
	user := User{"", 28, "Luis", "luis@vega.com", "", "Ateneo de Manila", Address{Line1: "79"}}
	err = pool.Save(&user)
	if err != nil {
		t.Error(err)
	}
	id := user.Id

	// Set to nil values
	user = User{}

	// Fetch user
	err = pool.Fetch(&user, id)
	if err != nil {
		t.Error(err)
	}

	// Find users
	users := []User{}
	err = pool.Find(&users, "College:Ateneo de Manila")
	if err != nil {
		t.Error(err)
	}

	// Delete user
	ok, err := pool.Delete("User", id)
	if err != nil {
		t.Error(err)
	}

	if !ok {
		t.Error("Failed to delete")
	}
}
