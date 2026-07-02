package chat

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"

	"github.com/SteVio89/stevio-home/db/queries"
)

var adjectives = []string{
	"Amber", "Bold", "Brave", "Bright", "Calm", "Clever", "Cool", "Cosmic",
	"Cozy", "Crisp", "Curious", "Daring", "Eager", "Fair", "Fancy", "Fast",
	"Fierce", "Fine", "Fleet", "Fluffy", "Flying", "Free", "Fresh", "Gentle",
	"Glad", "Golden", "Grand", "Happy", "Hardy", "Hasty", "Hearty", "Hidden",
	"Honest", "Iron", "Jade", "Jolly", "Keen", "Kind", "Lively", "Lucky",
	"Lunar", "Marble", "Mellow", "Mighty", "Misty", "Neat", "Noble", "Nimble",
	"Olive", "Opal", "Painted", "Peppy", "Pine", "Plucky", "Polite", "Proud",
	"Quick", "Quiet", "Radiant", "Rapid", "Rosy", "Ruby", "Rustic", "Sandy",
	"Scarlet", "Secret", "Sharp", "Shiny", "Silver", "Snowy", "Solar", "Steady",
	"Stone", "Storm", "Swift", "Tawny", "Tender", "Tiny", "Vivid", "Warm",
}

var nouns = []string{
	"Badger", "Bear", "Birch", "Bison", "Brook", "Cedar", "Cloud", "Comet",
	"Coral", "Crane", "Creek", "Dawn", "Deer", "Dingo", "Dolphin", "Dove",
	"Eagle", "Elm", "Falcon", "Fern", "Finch", "Flame", "Fox", "Gale",
	"Grove", "Hawk", "Heron", "Hill", "Ivy", "Jade", "Jay", "Lark",
	"Leaf", "Lily", "Lynx", "Maple", "Mars", "Meadow", "Moon", "Moss",
	"Oak", "Orca", "Otter", "Owl", "Panda", "Pearl", "Pebble", "Pine",
	"Plum", "Pond", "Quail", "Rain", "Raven", "Reed", "Ridge", "River",
	"Robin", "Rose", "Sage", "Sky", "Spark", "Star", "Stone", "Storm",
	"Swallow", "Swan", "Thorn", "Tiger", "Trail", "Trout", "Vale", "Violet",
	"Wave", "Willow", "Wolf", "Wren", "Yarrow", "Zephyr", "Zinnia", "Spruce",
}

func (h *ChatHandler) generateDisplayName(ctx context.Context, db *sql.DB) (string, error) {
	for i := 0; i < 5; i++ {
		adj := adjectives[rand.IntN(len(adjectives))]
		noun := nouns[rand.IntN(len(nouns))]
		name := adj + " " + noun
		exists, err := queries.ActiveDisplayNameExists(ctx, db, name)
		if err != nil {
			return "", err
		}
		if !exists {
			return name, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique display name after 5 attempts")
}
