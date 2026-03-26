package cart

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type Item struct {
	ProductID   string `json:"product_id"`
	UPC         string `json:"upc"`
	Name        string `json:"name,omitempty"`
	Quantity    int    `json:"quantity"`
	Modality    string `json:"modality"`
	AddedAt     string `json:"added_at"`
	LastUpdated string `json:"last_updated"`
}

type CartState struct {
	CurrentCart          []Item  `json:"current_cart"`
	LastUpdated          string  `json:"last_updated"`
	PreferredLocationID  *string `json:"preferred_location_id"`
}

type Order struct {
	Items     []Item `json:"items"`
	PlacedAt  string `json:"placed_at"`
	ItemCount int    `json:"item_count"`
}

type OrderHistory struct {
	Orders []Order `json:"orders"`
}

type LocalCart struct {
	mu          sync.Mutex
	cartFile    string
	historyFile string
}

func NewLocalCart(cartFile, historyFile string) *LocalCart {
	return &LocalCart{
		cartFile:    cartFile,
		historyFile: historyFile,
	}
}

func (lc *LocalCart) load() (*CartState, error) {
	data, err := os.ReadFile(lc.cartFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &CartState{CurrentCart: []Item{}}, nil
		}
		return nil, err
	}
	var state CartState
	if err := json.Unmarshal(data, &state); err != nil {
		return &CartState{CurrentCart: []Item{}}, nil
	}
	return &state, nil
}

func (lc *LocalCart) save(state *CartState) error {
	state.LastUpdated = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(lc.cartFile, data, 0644)
}

// AddItem adds or increments an item in the local cart.
func (lc *LocalCart) AddItem(productID, upc, name string, quantity int, modality string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	state, err := lc.load()
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)

	for i, item := range state.CurrentCart {
		if item.ProductID == productID && item.Modality == modality {
			state.CurrentCart[i].Quantity += quantity
			state.CurrentCart[i].LastUpdated = now
			return lc.save(state)
		}
	}

	state.CurrentCart = append(state.CurrentCart, Item{
		ProductID:   productID,
		UPC:         upc,
		Name:        name,
		Quantity:    quantity,
		Modality:    modality,
		AddedAt:     now,
		LastUpdated: now,
	})

	return lc.save(state)
}

// ViewCart returns the current cart state.
func (lc *LocalCart) ViewCart() (*CartState, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return lc.load()
}

// RemoveItem removes an item from the local cart.
func (lc *LocalCart) RemoveItem(productID, modality string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	state, err := lc.load()
	if err != nil {
		return err
	}

	filtered := make([]Item, 0, len(state.CurrentCart))
	for _, item := range state.CurrentCart {
		if item.ProductID == productID && (modality == "" || item.Modality == modality) {
			continue
		}
		filtered = append(filtered, item)
	}
	state.CurrentCart = filtered
	return lc.save(state)
}

// ClearCart empties the local cart.
func (lc *LocalCart) ClearCart() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	state, err := lc.load()
	if err != nil {
		return err
	}
	state.CurrentCart = []Item{}
	return lc.save(state)
}

// MarkOrderPlaced moves the current cart to order history.
func (lc *LocalCart) MarkOrderPlaced() error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	state, err := lc.load()
	if err != nil {
		return err
	}

	if len(state.CurrentCart) == 0 {
		return fmt.Errorf("cart is empty, nothing to mark as ordered")
	}

	history := lc.loadHistory()
	order := Order{
		Items:     state.CurrentCart,
		PlacedAt:  time.Now().Format(time.RFC3339),
		ItemCount: len(state.CurrentCart),
	}
	history.Orders = append(history.Orders, order)

	if err := lc.saveHistory(&history); err != nil {
		return err
	}

	state.CurrentCart = []Item{}
	return lc.save(state)
}

// ViewOrderHistory returns past orders.
func (lc *LocalCart) ViewOrderHistory(limit int) (*OrderHistory, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	history := lc.loadHistory()
	if limit > 0 && len(history.Orders) > limit {
		start := len(history.Orders) - limit
		history.Orders = history.Orders[start:]
	}
	return &history, nil
}

func (lc *LocalCart) loadHistory() OrderHistory {
	data, err := os.ReadFile(lc.historyFile)
	if err != nil {
		return OrderHistory{Orders: []Order{}}
	}
	var history OrderHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return OrderHistory{Orders: []Order{}}
	}
	return history
}

func (lc *LocalCart) saveHistory(history *OrderHistory) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(lc.historyFile, data, 0644)
}

// SetPreferredLocation persists the preferred store.
func (lc *LocalCart) SetPreferredLocation(locationID string) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	state, err := lc.load()
	if err != nil {
		return err
	}
	state.PreferredLocationID = &locationID
	return lc.save(state)
}

// GetPreferredLocation returns the preferred store ID.
func (lc *LocalCart) GetPreferredLocation() (string, error) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	state, err := lc.load()
	if err != nil {
		return "", err
	}
	if state.PreferredLocationID == nil {
		return "", nil
	}
	return *state.PreferredLocationID, nil
}
