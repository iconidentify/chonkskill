// Package fredmeyer provides a chonkskill for Fred Meyer / Kroger grocery
// shopping. It registers tools for product search, store lookup, cart
// management, and authentication, plus a SKILL.md with procedural knowledge
// for recipe-to-cart workflows, price comparison, meal planning, and more.
package fredmeyer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/iconidentify/chonkskill/pkg/skill"
	"github.com/iconidentify/chonkskill/skills/fredmeyer/internal/auth"
	"github.com/iconidentify/chonkskill/skills/fredmeyer/internal/cart"
	"github.com/iconidentify/chonkskill/skills/fredmeyer/internal/kroger"
)

//go:embed skill.md
var SkillContent string

//go:embed agent.md
var AgentContent string

// Config holds the credentials and settings for the Fred Meyer skill.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	BaseURL      string
	ZipCode      string
	DataDir      string
}

// ConfigFromEnv loads configuration from environment variables.
func ConfigFromEnv() Config {
	return Config{
		ClientID:     os.Getenv("KROGER_CLIENT_ID"),
		ClientSecret: os.Getenv("KROGER_CLIENT_SECRET"),
		RedirectURI:  envDefault("KROGER_REDIRECT_URI", "http://localhost:8000/callback"),
		BaseURL:      envDefault("KROGER_API_BASE", "https://api.kroger.com/v1"),
		ZipCode:      os.Getenv("KROGER_USER_ZIP_CODE"),
		DataDir:      envDefault("KROGER_DATA_DIR", "."),
	}
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// New creates the Fred Meyer skill with all tools registered.
func New(cfg Config) (*skill.Skill, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("KROGER_CLIENT_ID and KROGER_CLIENT_SECRET are required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.kroger.com/v1"
	}
	if cfg.RedirectURI == "" {
		cfg.RedirectURI = "http://localhost:8000/callback"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "."
	}

	client := kroger.NewClient(kroger.ClientConfig{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURI:  cfg.RedirectURI,
		BaseURL:      cfg.BaseURL,
	})
	am := auth.NewAuthManager(client, cfg.DataDir+"/kroger_token_user.json")
	lc := cart.NewLocalCart(cfg.DataDir+"/kroger_cart.json", cfg.DataDir+"/kroger_order_history.json")

	s := skill.New(skill.Definition{
		Name:         "fredmeyer",
		Description:  "Fred Meyer grocery shopping -- search products, manage cart, find stores, compare prices, build lists from recipes",
		SkillContent: SkillContent,
		AgentContent: AgentContent,
		Tags:         []string{"grocery", "shopping", "fredmeyer", "kroger", "cart", "recipes"},
	})

	registerLocationTools(s, client, lc, cfg.ZipCode)
	registerProductTools(s, client, lc)
	registerCartTools(s, client, am, lc)
	registerAuthTools(s, am)
	registerInfoTools(s, client, am)
	registerUtilityTools(s, cfg.ZipCode)

	return s, nil
}

// Register is the one-liner for chonkbase integration.
func Register(reg skill.Registry, cfg Config) error {
	s, err := New(cfg)
	if err != nil {
		return err
	}
	return s.Register(reg)
}

// --- Location Tools ---

func registerLocationTools(s *skill.Skill, client *kroger.Client, lc *cart.LocalCart, defaultZip string) {
	skill.AddTool(s, "search_locations",
		"Find Fred Meyer/Kroger stores near a zip code with hours, departments, and address",
		func(ctx context.Context, args SearchLocationsArgs) (string, error) {
			zip := args.ZipCode
			if zip == "" {
				zip = defaultZip
			}
			radius := args.RadiusMiles
			if radius == 0 {
				radius = 10
			}
			limit := args.Limit
			if limit == 0 {
				limit = 10
			}
			data, err := client.SearchLocations(zip, radius, limit, args.Chain)
			if err != nil {
				return "", err
			}
			return string(data), nil
		})

	skill.AddTool(s, "get_location_details",
		"Get detailed info about a specific store including hours, departments, and address",
		func(ctx context.Context, args LocationIDArgs) (string, error) {
			if args.LocationID == "" {
				return "", fmt.Errorf("location_id is required")
			}
			data, err := client.GetLocationDetails(args.LocationID)
			if err != nil {
				return "", err
			}
			return string(data), nil
		})

	skill.AddTool(s, "set_preferred_location",
		"Set your preferred Fred Meyer store for product searches. Prices are store-specific.",
		func(ctx context.Context, args LocationIDArgs) (string, error) {
			if err := lc.SetPreferredLocation(args.LocationID); err != nil {
				return "", err
			}
			return fmt.Sprintf("Preferred location set to %s", args.LocationID), nil
		})

	skill.AddTool(s, "get_preferred_location",
		"Get your currently set preferred store",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			locID, err := lc.GetPreferredLocation()
			if err != nil {
				return "", err
			}
			if locID == "" {
				return "No preferred location set. Use fredmeyer:set_preferred_location to set one.", nil
			}
			return fmt.Sprintf("Preferred location: %s", locID), nil
		})
}

// --- Product Tools ---

func registerProductTools(s *skill.Skill, client *kroger.Client, lc *cart.LocalCart) {
	skill.AddTool(s, "search_products",
		"Search for products at a Fred Meyer/Kroger store. Returns pricing, availability, aisle location, and size. Include a location_id for store-specific pricing.",
		func(ctx context.Context, args SearchProductsArgs) (string, error) {
			locID := args.LocationID
			if locID == "" {
				if pref, _ := lc.GetPreferredLocation(); pref != "" {
					locID = pref
				}
			}
			limit := args.Limit
			if limit == 0 {
				limit = 10
			}
			data, err := client.SearchProducts(args.Term, locID, limit, args.Brand, args.Fulfillment)
			if err != nil {
				return "", err
			}
			return string(data), nil
		})

	skill.AddTool(s, "get_product_details",
		"Get full details for a specific product including all sizes, images, and pricing",
		func(ctx context.Context, args ProductDetailsArgs) (string, error) {
			locID := args.LocationID
			if locID == "" {
				if pref, _ := lc.GetPreferredLocation(); pref != "" {
					locID = pref
				}
			}
			data, err := client.GetProductDetails(args.ProductID, locID)
			if err != nil {
				return "", err
			}
			return string(data), nil
		})
}

// --- Cart Tools ---

func registerCartTools(s *skill.Skill, client *kroger.Client, am *auth.AuthManager, lc *cart.LocalCart) {
	skill.AddTool(s, "add_to_cart",
		"Add an item to your Kroger cart by UPC. Requires authentication. Also tracks locally for cart view.",
		func(ctx context.Context, args AddToCartArgs) (string, error) {
			if !am.IsAuthenticated() {
				return "", fmt.Errorf("not authenticated -- use fredmeyer:start_authentication first")
			}
			quantity := args.Quantity
			if quantity == 0 {
				quantity = 1
			}
			productID := args.ProductID
			if productID == "" {
				productID = args.UPC
			}
			modality := args.Modality
			if modality == "" {
				modality = "PICKUP"
			}

			err := client.AddToCart([]kroger.CartItem{{UPC: args.UPC, Quantity: quantity}})
			if err != nil {
				return "", fmt.Errorf("Kroger API error: %w", err)
			}

			if err := lc.AddItem(productID, args.UPC, args.Name, quantity, modality); err != nil {
				slog.Warn("local cart tracking failed", "error", err)
			}

			return fmt.Sprintf("Added %d x %s (UPC: %s) to cart", quantity, args.Name, args.UPC), nil
		})

	skill.AddTool(s, "view_cart",
		"View current cart contents. NOTE: This shows locally tracked items only, not the actual Kroger cart.",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			state, err := lc.ViewCart()
			if err != nil {
				return "", err
			}
			if len(state.CurrentCart) == 0 {
				return "Cart is empty.", nil
			}
			b, _ := json.MarshalIndent(state, "", "  ")
			return string(b), nil
		})

	skill.AddTool(s, "remove_from_cart",
		"Remove an item from local cart tracking. NOTE: Does NOT remove from the actual Kroger cart.",
		func(ctx context.Context, args RemoveFromCartArgs) (string, error) {
			if err := lc.RemoveItem(args.ProductID, args.Modality); err != nil {
				return "", err
			}
			return fmt.Sprintf("Removed %s from local cart tracking", args.ProductID), nil
		})

	skill.AddTool(s, "clear_cart",
		"Clear all items from local cart tracking. NOTE: Does NOT clear the actual Kroger cart.",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			if err := lc.ClearCart(); err != nil {
				return "", err
			}
			return "Local cart cleared.", nil
		})

	skill.AddTool(s, "mark_order_placed",
		"Mark the current cart as ordered and archive to order history. Call after the user completes checkout on fredmeyer.com.",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			if err := lc.MarkOrderPlaced(); err != nil {
				return "", err
			}
			return "Order marked as placed and moved to history.", nil
		})

	skill.AddTool(s, "view_order_history",
		"View past orders placed through this assistant",
		func(ctx context.Context, args ViewOrderHistoryArgs) (string, error) {
			limit := args.Limit
			if limit == 0 {
				limit = 10
			}
			history, err := lc.ViewOrderHistory(limit)
			if err != nil {
				return "", err
			}
			if len(history.Orders) == 0 {
				return "No order history.", nil
			}
			b, _ := json.MarshalIndent(history, "", "  ")
			return string(b), nil
		})
}

// --- Auth Tools ---

func registerAuthTools(s *skill.Skill, am *auth.AuthManager) {
	skill.AddTool(s, "start_authentication",
		"Start OAuth2 login flow. Returns a URL the user must visit to authorize access to their Kroger account.",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			url, err := am.StartAuth("product.compact cart.basic:write profile.compact")
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("Please visit this URL to authorize:\n\n%s\n\nAfter signing in, Kroger will redirect you. Paste the full redirect URL back here.", url), nil
		})

	skill.AddTool(s, "complete_authentication",
		"Complete OAuth2 login by providing the redirect URL after the user authorizes",
		func(ctx context.Context, args RedirectURLArgs) (string, error) {
			if args.RedirectURL == "" {
				return "", fmt.Errorf("redirect_url is required")
			}
			if err := am.CompleteAuth(args.RedirectURL); err != nil {
				return "", err
			}
			return "Authentication successful. You can now add items to cart and access your profile.", nil
		})

	skill.AddTool(s, "test_authentication",
		"Check if the current Kroger authentication is valid",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			if am.IsAuthenticated() {
				return "Authenticated and token is valid.", nil
			}
			return "Not authenticated. Use fredmeyer:start_authentication to begin.", nil
		})

	skill.AddTool(s, "force_reauthenticate",
		"Invalidate current auth tokens and require re-authentication",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			am.ForceReauth()
			return "Auth tokens invalidated. Use fredmeyer:start_authentication to re-authenticate.", nil
		})
}

// --- Info Tools ---

func registerInfoTools(s *skill.Skill, client *kroger.Client, am *auth.AuthManager) {
	skill.AddTool(s, "list_chains",
		"List all Kroger-owned chains (Fred Meyer, QFC, Ralphs, King Soopers, etc.)",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			data, err := client.GetChains()
			if err != nil {
				return "", err
			}
			return string(data), nil
		})

	skill.AddTool(s, "list_departments",
		"List all store departments with IDs",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			data, err := client.GetDepartments()
			if err != nil {
				return "", err
			}
			return string(data), nil
		})

	skill.AddTool(s, "get_user_profile",
		"Get the authenticated user's Kroger profile ID",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			if !am.IsAuthenticated() {
				return "", fmt.Errorf("not authenticated -- use fredmeyer:start_authentication first")
			}
			data, err := client.GetProfile()
			if err != nil {
				return "", err
			}
			return string(data), nil
		})
}

// --- Utility Tools ---

func registerUtilityTools(s *skill.Skill, zipCode string) {
	skill.AddTool(s, "get_zip_code",
		"Get the configured default zip code",
		func(ctx context.Context, args EmptyArgs) (string, error) {
			if zipCode == "" {
				return "No zip code configured. Set KROGER_USER_ZIP_CODE env var.", nil
			}
			return zipCode, nil
		})
}

