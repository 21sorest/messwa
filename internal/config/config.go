// Package config loads messwa's TOML configuration.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	StateDir string `toml:"state_dir"`
	Timezone string `toml:"timezone"`

	// Groups are the mess buy/sell group JIDs (xxxx@g.us) to merge.
	Groups []string `toml:"groups"`

	Meals  []Meal `toml:"meals"`
	Messes []Mess `toml:"messes"`
	Dedupe Dedupe `toml:"dedupe"`
	Sold   Sold   `toml:"sold"`
	DMs    DMs    `toml:"dms"`
	MCP    MCP    `toml:"mcp"`

	loc *time.Location
}

type Meal struct {
	Name     string   `toml:"name"`
	Starts   string   `toml:"starts"` // "HH:MM"
	Ends     string   `toml:"ends"`
	Keywords []string `toml:"keywords"`
}

type Mess struct {
	Name    string   `toml:"name"`
	Aliases []string `toml:"aliases"`
}

type Dedupe struct {
	// Posts by the same person for the same meal slot within this window
	// are one listing.
	Window duration `toml:"window"`
}

type Sold struct {
	Emojis []string `toml:"emojis"`
	// Who can mark a listing sold by reacting: the seller always can.
	// Admins of the group can too when AdminsCount is true.
	AdminsCount bool `toml:"admins_count"`
	// Words that mark a listing sold when the seller edits the post to
	// include them or replies to it with them.
	Words []string `toml:"words"`
}

type DMs struct {
	// How long DM content from unsaved numbers is kept.
	Retention duration `toml:"retention"`
}

type MCP struct {
	Listen string `toml:"listen"`
	// Name of the env var holding a bearer token. Empty disables auth,
	// which is only acceptable when Listen is loopback.
	TokenEnv string `toml:"token_env"`
}

type duration struct{ time.Duration }

func (d *duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	d.Duration = v
	return err
}

func Load(path string) (*Config, error) {
	c := Default()
	if _, err := toml.DecodeFile(path, c); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return c, c.validate()
}

func Default() *Config {
	return &Config{
		StateDir: "state",
		Timezone: "Asia/Kolkata",
		Meals: []Meal{
			{Name: "breakfast", Starts: "07:30", Ends: "09:30", Keywords: []string{"breakfast", "bf", "b/f", "bfast", "nashta"}},
			{Name: "lunch", Starts: "12:30", Ends: "14:30", Keywords: []string{"lunch"}},
			{Name: "snacks", Starts: "16:45", Ends: "18:15", Keywords: []string{"snacks", "snack"}},
			{Name: "dinner", Starts: "19:30", Ends: "21:30", Keywords: []string{"dinner"}},
		},
		Messes: []Mess{
			{Name: "kadamba", Aliases: []string{"kadamb", "kadamba"}},
			{Name: "yuktahar", Aliases: []string{"yuktahar", "yukta"}},
			{Name: "palash", Aliases: []string{"palash"}},
		},
		Dedupe: Dedupe{Window: duration{3 * time.Hour}},
		Sold: Sold{
			Emojis:      []string{"✅", "✔️", "☑️", "✔"},
			AdminsCount: true,
			Words:       []string{"sold", "done", "gone"},
		},
		DMs: DMs{Retention: duration{72 * time.Hour}},
		MCP: MCP{Listen: "127.0.0.1:8765", TokenEnv: "MESSWA_MCP_TOKEN"},
	}
}

func (c *Config) validate() error {
	loc, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return fmt.Errorf("timezone: %w", err)
	}
	c.loc = loc
	for _, m := range c.Meals {
		if _, err := ParseClock(m.Starts); err != nil {
			return fmt.Errorf("meal %s starts: %w", m.Name, err)
		}
		if _, err := ParseClock(m.Ends); err != nil {
			return fmt.Errorf("meal %s ends: %w", m.Name, err)
		}
	}
	return nil
}

func (c *Config) Location() *time.Location {
	if c.loc == nil {
		c.loc, _ = time.LoadLocation(c.Timezone)
	}
	return c.loc
}

// ParseClock parses "HH:MM" into minutes after midnight.
func ParseClock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, err
	}
	return t.Hour()*60 + t.Minute(), nil
}

func (c *Config) SessionDB() string { return c.StateDir + "/session.db" }
func (c *Config) DataDB() string    { return c.StateDir + "/messwa.db" }

func EnsureDir(dir string) error { return os.MkdirAll(dir, 0o700) }
