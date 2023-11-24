package main

import (
	"fmt"
	"testing"
)

// func TestOrderBook(t *testing.T) {

// }

func TestLimit(t *testing.T) {
	l := NewLimit(10_000)
	buyOrderA := NewOrder(true, 5)
	buyOrderB := NewOrder(true, 8)
	buyOrderC := NewOrder(true, 10)

	l.AddOrder(buyOrderA)
	l.AddOrder(buyOrderB)
	l.AddOrder(buyOrderC)

	l.DeleteOrder(buyOrderB)

	buyOrder := NewOrder(true, 5)
	l.AddOrder(buyOrder)
	fmt.Println(l)
}

func TestOrderBook(t *testing.T) {
	ob := NewOrderBook()
	buyOrder := NewOrder(true, 10)
	ob.PlaceOrder(10_000, buyOrder)

	fmt.Printf("%+v", ob.Bids[0])
}
