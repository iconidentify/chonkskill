---
name: fred-meyer-grocery
description: >
  Search Fred Meyer and Kroger products with real-time pricing, manage a grocery cart,
  find nearby stores, compare prices across brands and sizes, build shopping lists from
  recipes, and plan budget-aware meals. Use when the user mentions groceries, shopping,
  Fred Meyer, Kroger, meal planning, recipes, food, cooking ingredients, grocery budget,
  price comparison, or wants to add items to a cart. Also activate when the user asks
  about store hours, product availability, aisle locations, or dietary filtering of
  grocery products.
license: MIT
compatibility: Requires KROGER_CLIENT_ID and KROGER_CLIENT_SECRET environment variables. Kroger developer account at developer.kroger.com.
metadata:
  author: chonkskill
  version: "1.0"
  origin: iconidentify/chonkskill
  requires_tools:
    - fredmeyer:search_locations
    - fredmeyer:get_location_details
    - fredmeyer:set_preferred_location
    - fredmeyer:get_preferred_location
    - fredmeyer:search_products
    - fredmeyer:get_product_details
    - fredmeyer:add_to_cart
    - fredmeyer:view_cart
    - fredmeyer:remove_from_cart
    - fredmeyer:clear_cart
    - fredmeyer:mark_order_placed
    - fredmeyer:view_order_history
    - fredmeyer:start_authentication
    - fredmeyer:complete_authentication
    - fredmeyer:test_authentication
    - fredmeyer:list_chains
    - fredmeyer:list_departments
    - fredmeyer:get_user_profile
    - fredmeyer:get_zip_code
---

# Fred Meyer Grocery Shopping

Help users browse Fred Meyer stores, search products with real-time store pricing,
manage a shopping cart, compare prices, plan meals, and build grocery lists from
recipes -- all through the Kroger API.

Fred Meyer is a Kroger subsidiary. All tools use the official Kroger Public API
and work with any Kroger-family store (QFC, Ralphs, King Soopers, etc.), though
the default experience is tuned for Fred Meyer.

---

## Quick Reference

| Task | Tool | Auth Required |
|------|------|---------------|
| Find stores | `fredmeyer:search_locations` | No |
| Set default store | `fredmeyer:set_preferred_location` | No |
| Search products | `fredmeyer:search_products` | No |
| Product detail | `fredmeyer:get_product_details` | No |
| Add to cart | `fredmeyer:add_to_cart` | Yes |
| View cart | `fredmeyer:view_cart` | No |
| Begin login | `fredmeyer:start_authentication` | No |
| Complete login | `fredmeyer:complete_authentication` | No |

---

## First-Time Setup

A preferred store must be set before product searches return useful results.
Without a location ID, searches return products without pricing or availability.

1. Get the user's zip code, or call `fredmeyer:get_zip_code` for the configured default
2. Call `fredmeyer:search_locations` with that zip code
3. Present the nearest stores -- highlight Fred Meyer locations (chain contains "FRED")
4. Ask the user which store they prefer
5. Call `fredmeyer:set_preferred_location` with the chosen `locationId`

After this, all product searches automatically use the preferred store for
pricing and availability. This persists across sessions.

---

## Searching for Products

Call `fredmeyer:search_products` with a `term` parameter. Be specific -- the
API does full-text search and vague terms return noisy results.

Good search terms: "organic whole milk half gallon", "honeycrisp apples",
"tillamook sharp cheddar". Avoid: "milk" alone (too broad), "food" (meaningless).

### Reading Results

Each product result contains:
- `productId` and `upc` -- needed for cart operations
- `description` and `brand` -- what the product is
- `items[].price.regular` -- the shelf price
- `items[].price.promo` -- the sale price (0 if not on sale)
- `items[].size` -- package size ("64 fl oz", "1 lb", etc.)
- `aisleLocations[].description` -- where to find it in the store
- `aisleLocations[].number` -- specific aisle number

### Filtering

- `brand`: filter by brand name (exact match)
- `fulfillment`: filter by availability type
  - `ais` = available in store
  - `csp` = available for pickup
  - `dth` = available for delivery

### Price Display

When presenting products to the user, always show:
- The regular price
- The sale price if one exists (with the savings amount)
- The package size
- Calculate and show the unit price (price / size) when comparing similar products

---

## Price Comparison

Price comparison is one of the most valuable things you can do. The Kroger API
returns both regular and promo pricing, plus package sizes, which lets you
calculate unit prices.

### Comparing Across Brands

When the user wants to compare options for a product:

1. Search with a general term (e.g., "cheddar cheese")
2. For each result, calculate unit price: `price / quantity` in a common unit
3. Present a comparison table sorted by unit price, like:

```
Brand              | Size   | Price  | Unit Price | On Sale?
Simple Truth       | 8 oz   | $3.49  | $0.44/oz   | No
Tillamook          | 8 oz   | $4.99  | $0.62/oz   | Yes ($3.99 = $0.50/oz)
Kroger             | 16 oz  | $5.99  | $0.37/oz   | No
```

The larger package is not always cheaper per unit. Surface this when it happens --
it is a genuinely useful insight most shoppers miss.

### Comparing Across Stores

The user can compare prices at different stores by searching with different
`location_id` values:

1. Call `fredmeyer:search_locations` to get nearby stores
2. Search the same product at each store
3. Compare pricing -- Fred Meyer and QFC are both Kroger stores but may have
   different pricing on the same item

---

## Building a Grocery List from a Recipe

This is the highest-value workflow. The user provides a recipe (URL, text, or
description) and you convert it into a priced shopping list.

### Procedure

1. Parse the recipe into a structured ingredient list with quantities and units
2. For each ingredient, determine the best search term:
   - "2 cups chicken broth" -> search "chicken broth"
   - "1 lb boneless skinless chicken breast" -> search "boneless skinless chicken breast"
   - "salt and pepper to taste" -> skip (pantry staple, ask user if needed)
3. Call `fredmeyer:search_products` for each ingredient
4. For each search result set, select the best match considering:
   - Closest quantity to what the recipe needs (avoid buying 48 oz when 16 oz suffices)
   - Reasonable price
   - In-stock status
5. Present the full list with running total:

```
Recipe: Chicken Piccata (serves 4)

Item                              | Product Match           | Size   | Price
1 lb chicken breast               | Kroger Boneless Breast  | 1 lb   | $5.99
2 lemons                          | Organic Lemons (each)   | 1 ct   | $0.79 x2
3 tbsp capers                     | Kroger Capers           | 3.5 oz | $2.49
1/2 cup chicken broth             | Swanson Chicken Broth   | 14 oz  | $1.99
1/4 cup flour                     | Kroger All Purpose Flour| 5 lb   | $3.29
2 tbsp butter                     | Kroger Salted Butter    | 16 oz  | $4.49

Estimated Total: $19.84
```

6. Ask: "Do you already have any of these? I can remove pantry staples."
7. After confirmation, add items to cart if the user is authenticated

### Quantity Intelligence

The recipe-to-cart quantity problem is real. When a recipe calls for "2 tbsp
butter" and the smallest package is a 16 oz stick, note this:

"The recipe needs 2 tbsp butter. The smallest available is a 16 oz package
($4.49). You will have plenty left over. Skip this if you already have butter
at home."

This prevents the most common complaint about recipe-to-cart features: buying
full packages of items you only need a small amount of.

### Multi-Recipe Aggregation

When planning multiple meals, aggregate ingredients across recipes before
searching:

1. Collect all ingredients from all recipes
2. Combine duplicates: if Recipe A needs 1 lb chicken and Recipe B needs 0.5 lb,
   search for 1.5 lb chicken -- not two separate items
3. Present the consolidated list with which recipes each item serves
4. This is the single most time-saving feature in meal planning

---

## Meal Planning

When the user wants to plan meals for a period (typically a week):

### Budget-Aware Planning

1. Ask for: number of meals, number of people, budget, dietary restrictions
2. Suggest meals that fit the constraints
3. For each proposed meal, estimate cost by searching key ingredients
4. Adjust the plan until total fits the budget
5. Generate the consolidated grocery list (see multi-recipe aggregation above)
6. Present with total cost and per-meal breakdown

### Seasonal Awareness

Produce that is in season is both cheaper and better quality. When suggesting
recipes or ingredients:

- Spring (Mar-May): asparagus, strawberries, peas, artichokes
- Summer (Jun-Aug): tomatoes, corn, berries, stone fruit, zucchini
- Fall (Sep-Nov): squash, apples, pears, sweet potatoes, Brussels sprouts
- Winter (Dec-Feb): citrus, root vegetables, kale, cabbage

Bias suggestions toward in-season produce. When a recipe calls for an
out-of-season item, mention it: "This recipe uses tomatoes, which are out of
season and will be more expensive. Consider substituting canned San Marzano
tomatoes for better flavor and price."

---

## Dietary Filtering

When the user has dietary restrictions, apply them consistently to every
product search and recommendation.

### How to Filter

The Kroger API does not have a dietary filter parameter. You must filter
results by reading product descriptions and brand names:

- **Gluten-free**: Look for "gluten free" in description or brand (Simple Truth Gluten Free line)
- **Organic**: Look for "organic" in description or use `brand` filter with "Simple Truth Organic"
- **Vegan/Plant-based**: Look for "plant based", "vegan", or known vegan brands
- **Keto/Low-carb**: Check product category and known keto brands
- **Allergen-specific**: Read descriptions carefully for allergen callouts

### Persistence

When a user states a dietary restriction, remember it and apply it to all
subsequent searches in the session. Do not make the user repeat it. If
suggesting a substitution, verify the substitute also meets the restriction.

---

## Shopping Cart Operations

### Adding Items

Cart operations require OAuth2 authentication with the user's Kroger account.

If the user is not authenticated and tries to add items:
1. Call `fredmeyer:start_authentication` -- this returns an authorization URL
2. Tell the user: "To add items to your Kroger cart, you need to sign in. Please visit this URL and authorize the app, then paste the redirect URL back here."
3. When they paste the redirect URL, call `fredmeyer:complete_authentication`
4. Proceed with cart additions

To add an item, call `fredmeyer:add_to_cart` with the `upc` from the product
search results. Also pass `product_id`, `name`, and `quantity` for local tracking.

### Cart Limitations

These are fundamental API constraints that cannot be worked around:

- **Add-only**: The Kroger API can add items but cannot remove them or read the real cart
- **No cart view**: `fredmeyer:view_cart` shows locally tracked items only
- **No removal**: `fredmeyer:remove_from_cart` removes from local tracking only
- **No checkout**: There is no checkout API -- the user must complete checkout on fredmeyer.com or the Kroger app
- **Cart drift**: If the user modifies their cart through the Kroger app or website, local tracking will be out of sync

Always be transparent about these limitations. When showing the cart, note:
"This shows items added through this assistant. Your actual Kroger cart may
contain additional items added through the app or website."

### After Shopping

When the user confirms they have placed their order:
1. Call `fredmeyer:mark_order_placed` to archive the cart to order history
2. This clears the local cart and saves the order with a timestamp
3. The user can review past orders with `fredmeyer:view_order_history`

---

## Substitution Logic

When a searched product is unavailable or out of stock, suggest alternatives
using this priority order:

1. **Same product, different size** -- closest to needed quantity
2. **Same brand, similar product** -- brand loyalty matters for some categories
3. **Store brand equivalent** -- Kroger/Simple Truth brands are usually cheaper
4. **Different brand, same category** -- search with broader terms
5. **Functional substitute** -- different product that serves the same role
   (Greek yogurt for sour cream, applesauce for oil in baking)

For each substitution, explain why you chose it and show the price difference.
Never silently substitute -- always tell the user what changed and why.

---

## Replenishment and Recurring Items

When the user has order history, you can suggest replenishment:

1. Call `fredmeyer:view_order_history` to see past purchases
2. Identify items that appear frequently
3. Ask: "Based on your history, you usually buy [items]. Want me to add these
   to your cart?"

This is particularly useful for staples: milk, eggs, bread, coffee, etc.

---

## Troubleshooting

### "Not authenticated" error
The user needs to log in. Call `fredmeyer:start_authentication` and walk them
through the OAuth flow.

### No prices in search results
The preferred location is not set, or the `location_id` was not passed.
Call `fredmeyer:get_preferred_location` to check, and set one if missing.

### Token expired
Tokens last 30 minutes. The system auto-refreshes using the refresh token. If
refresh fails (token revoked or expired beyond refresh window), the user needs
to re-authenticate via `fredmeyer:start_authentication`.

### "Product not found" for a known item
Try different search terms. The Kroger search is full-text and sometimes
misses on exact product names. Try the brand name alone, or a more generic
description.

### Rate limits
The API has daily limits: 10,000 product searches, 5,000 cart adds, 1,600
location queries. Under normal use these are never hit. If you see rate limit
errors, reduce search frequency and batch cart adds.

---

## Pitfalls

- **Do not assume availability.** A product appearing in search results does not
  guarantee it is in stock at the store right now. The API reflects catalog data,
  not real-time shelf inventory.

- **Do not conflate local cart with real cart.** The local cart is a convenience
  tracker. The source of truth is the user's actual Kroger cart on fredmeyer.com.

- **Do not try to automate checkout.** There is no API for this. The user must
  visit fredmeyer.com or use the Kroger app to complete their order.

- **Do not search without a location.** Prices are store-specific. A search
  without a location returns products but no pricing. Always confirm the
  preferred location is set.

- **Do not ignore package sizes.** When building a recipe list, a search for
  "olive oil" might return a $12 bottle when the recipe needs one tablespoon.
  Flag the mismatch rather than silently adding an oversized purchase.
