package main

import (
	"fmt"
	"sort"
	"time"
)

type Match struct {
	Ask        *Order
	Bid        *Order
	SizeFilled float64
	Price      float64
}

type Order struct {
	Size      float64
	Bid       bool
	Limit     *Limit
	Timestamp int64
}

type Orders []*Order

func (o Orders) Len() int           { return len(o) }
func (o Orders) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o Orders) Less(i, j int) bool { return o[i].Timestamp < o[j].Timestamp }

func NewOrder(bid bool, size float64) *Order {
	return &Order{
		Size:      size,
		Bid:       bid,
		Timestamp: time.Now().UnixNano(),
	}
}

func (o *Order) String() string {
	return fmt.Sprintf("[size: %.2f]", o.Size)
}

func (o *Order) isFilled() bool {
	return o.Size == 0.0
}

type Limit struct {
	Price       float64
	Orders      Orders
	TotalVolume float64
}

type Limits []*Limit

type ByBestAsk struct{ Limits }

func (a ByBestAsk) Len() int           { return len(a.Limits) }
func (a ByBestAsk) Swap(i, j int)      { a.Limits[i], a.Limits[j] = a.Limits[j], a.Limits[i] }
func (a ByBestAsk) Less(i, j int) bool { return a.Limits[i].Price < a.Limits[j].Price }

type ByBestBid struct{ Limits }

func (b ByBestBid) Len() int           { return len(b.Limits) }
func (b ByBestBid) Swap(i, j int)      { b.Limits[i], b.Limits[j] = b.Limits[j], b.Limits[i] }
func (a ByBestBid) Less(i, j int) bool { return a.Limits[i].Price > a.Limits[j].Price }

func NewLimit(price float64) *Limit {
	return &Limit{
		Price:  price,
		Orders: []*Order{},
	}
}

func (l *Limit) AddOrder(o *Order) {
	o.Limit = l
	l.Orders = append(l.Orders, o)
	l.TotalVolume += o.Size
}

func (l *Limit) DeleteOrder(o *Order) {
	for i := 0; i < len(l.Orders); i++ {
		if l.Orders[i] == o {
			l.Orders[i] = l.Orders[len(l.Orders)-1]
			l.Orders = l.Orders[:len(l.Orders)-1]
		}
	}

	o.Limit = nil
	l.TotalVolume -= o.Size

	//TODO: resort the whole resting orders
	sort.Sort(l.Orders)
}

func (ob *Order) CancelOrder(o *Order) {
	limit := o.Limit
	limit.DeleteOrder(o)
}

func (l *Limit) Fill(o *Order) []Match {
	var (
		matches        []Match  // Store the matches resulting from filling the order
		ordersToDelete []*Order // Keep track of orders to delete after processing
	)

	for _, order := range l.Orders {
		match := l.fillOrder(order, o)
		matches = append(matches, match)

		l.TotalVolume -= match.SizeFilled

		if order.isFilled() {
			ordersToDelete = append(ordersToDelete, order)
		}

		if o.isFilled() {
			break
		}
	}

	for _, order := range ordersToDelete {
		l.DeleteOrder(order)
	}

	return matches
}

// TODO: Add more context to fillorder function, preferably with chatGPT
func (l *Limit) fillOrder(a, b *Order) Match {
	var (
		bid        *Order  // represent the bid order
		ask        *Order  // represent the ask order
		sizeFilled float64 // represent the filled size in the match
	)

	// determine the bid and ask size based on their bid field
	if a.Bid {
		bid = a
		ask = b
	} else {
		bid = b
		ask = a
	}

	// Compare the sizes of orders 'a' and 'b' to determine the filled size and adjust sizes accordingly
	if a.Size >= b.Size {
		// 'a' has a size greater than or equal to 'b'
		a.Size -= b.Size    // reduce "a" size by "b"  size
		sizeFilled = b.Size // record "b" size as the filled size
		b.Size = 0.0        // set "b" size to zero indicating complete fill or partial fill by "a"
	} else {
		b.Size -= a.Size    // reduce "b" size by "a" size
		sizeFilled = a.Size // record "a" size as the filled size
		a.Size = 0.0        // set "a" size to zero indicating complete fill or partial fill by "b"
	}

	return Match{
		Bid:        bid,
		Ask:        ask,
		SizeFilled: sizeFilled,
		Price:      l.Price,
	}
}

type OrderBook struct {
	asks []*Limit
	bids []*Limit

	AskLimits map[float64]*Limit
	BidLimits map[float64]*Limit
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		asks:      []*Limit{},
		bids:      []*Limit{},
		AskLimits: make(map[float64]*Limit),
		BidLimits: make(map[float64]*Limit),
	}
}

func (ob *OrderBook) PlaceMarketOrder(o *Order) []Match {
	matches := []Match{} // Initialize an empty slice to store matches

	// Check if the market order is a bid
	if o.Bid {
		// Check if the market order size is greater than the total volume of asks
		if o.Size > ob.AskTotalVolume() {
			// Raise a panic if the market order size exceeds available ask volume
			panic(fmt.Errorf("Not enough volume [%.2f] for market order [size: %.2f]", ob.AskTotalVolume(), o.Size))
		}
		// Iterate through the asks in the order book
		for _, limit := range ob.Asks() {
			// Fill the market order against the current ask limit and collect matches
			limitMatches := limit.Fill(o)
			matches = append(matches, limitMatches...)

			// Check if the current ask limit has no remaining orders and clear if no limit remaining
			if len(limit.Orders) == 0 {
				ob.clearLimit(true, limit)
			}
		}
	} else {
		// If the market order is not a bid (it's an ask)
		// Check if the market order size is greater than the total volume of bids
		if o.Size > ob.BidTotalVolume() {
			// Raise a panic if the market order size exceeds available bid volume
			panic(fmt.Errorf("Not enough volume [%.2f] for market order [size: %.2f]", ob.BidTotalVolume(), o.Size))
		}

		// Iterate through the bids in the order books
		for _, limit := range ob.Bids() {
			// Fill the market order against the current bid limit and collect matches
			limitMatches := limit.Fill(o)
			matches = append(matches, limitMatches...)

			// Check if the current bid limit has no remaining orders
			if len(limit.Orders) == 0 {
				// If no orders remain, clear the limit from the order book
				ob.clearLimit(true, limit)
			}
		}
	}

	return matches // Return the collected matches resulting from the market order execution
}

func (ob *OrderBook) PlaceLimitOrder(price float64, o *Order) {
	var limit *Limit
	// Determine whether the order is a bid or ask and retrieve the corresponding limit
	if o.Bid {
		limit = ob.BidLimits[price]
	} else {
		limit = ob.AskLimits[price]
	}

	// If the limit at the specified price does not exist, create a new limit
	if limit == nil {
		limit = NewLimit(price)

		// Assign the new limit to the corresponding side of the order book and update the associated maps
		if o.Bid {
			ob.bids = append(ob.bids, limit) // Add the new limit to the bids slice
			ob.BidLimits[price] = limit      // Update the BidLimits map with the new limit
		} else {
			ob.asks = append(ob.asks, limit) // Add the new limit to the asks slice
			ob.AskLimits[price] = limit      // Update the AskLimits map with the new limit
		}
	}

	limit.AddOrder(o) // Add the order to the identified or newly created limit
}

func (ob *OrderBook) clearLimit(bid bool, l *Limit) {
	// If the limit is on the bid side
	if bid {
		delete(ob.BidLimits, l.Price) // Remove the limit from BidLimits map using its price as the key
		// Find and remove the limit from the bids slice
		for i := 0; i < len(ob.bids); i++ {
			if ob.bids[i] == l {
				// Swap the current limit with the last one and reduce the slice by one
				ob.bids[i] = ob.bids[len(ob.bids)-1]
				ob.bids = ob.bids[:len(ob.bids)-1]
			}
		}
	} else {
		// If the limit is on the ask side
		delete(ob.AskLimits, l.Price) // Remove the limit from AskLimits map using its price as the key

		// Find and remove the limit from the asks slice
		for i := 0; i < len(ob.asks); i++ {
			if ob.asks[i] == l {
				// Swap the current limit with the last one and reduce the slice by one
				ob.asks[i] = ob.asks[len(ob.asks)-1]
				ob.asks = ob.asks[:len(ob.asks)-1]
			}
		}
	}
}

func (ob *OrderBook) BidTotalVolume() float64 {
	totalVolume := 0.0

	// Calculate the total volume by iterating through bid limits
	for i := 0; i < len(ob.bids); i++ {
		totalVolume += ob.bids[i].TotalVolume
	}

	return totalVolume
}

func (ob *OrderBook) AskTotalVolume() float64 {
	totalVolume := 0.0

	// Calculate the total volume by iterating through ask limits
	for i := 0; i < len(ob.asks); i++ {
		totalVolume += ob.asks[i].TotalVolume
	}

	return totalVolume
}

func (ob *OrderBook) Asks() []*Limit {
	sort.Sort(ByBestAsk{ob.asks})
	return ob.asks
}

func (ob *OrderBook) Bids() []*Limit {
	sort.Sort(ByBestBid{ob.bids})
	return ob.bids
}
