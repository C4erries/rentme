package pricing

import (
	"encoding/json"
	"log/slog"
	"strings"

	domainlistings "rentme/internal/domain/listings"
)

type ClampRange struct {
	MinRub int64 `json:"min_rub"`
	MaxRub int64 `json:"max_rub"`
}

type ClampConfig struct {
	Defaults map[domainlistings.RentalTermType]ClampRange                       `json:"defaults"`
	Cities   map[string]map[domainlistings.RentalTermType]ClampRange            `json:"cities"`
}

func DefaultClampConfig() ClampConfig {
	defaults := map[domainlistings.RentalTermType]ClampRange{
		domainlistings.RentalTermShort: {MinRub: 3_000, MaxRub: 30_000},
		domainlistings.RentalTermLong:  {MinRub: 25_000, MaxRub: 250_000},
	}
	return ClampConfig{
		Defaults: defaults,
		Cities: map[string]map[domainlistings.RentalTermType]ClampRange{
			"Москва": {
				domainlistings.RentalTermShort: {MinRub: 3_000, MaxRub: 35_000},
				domainlistings.RentalTermLong:  {MinRub: 25_000, MaxRub: 300_000},
			},
			"Краснодар": {
				domainlistings.RentalTermShort: {MinRub: 2_000, MaxRub: 25_000},
				domainlistings.RentalTermLong:  {MinRub: 20_000, MaxRub: 200_000},
			},
		},
	}
}

func LoadClampConfig(raw string, logger *slog.Logger) ClampConfig {
	if strings.TrimSpace(raw) == "" {
		return DefaultClampConfig()
	}

	var cfg ClampConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		if logger != nil {
			logger.Warn("invalid ML_PRICE_CLAMPS JSON, using defaults", "error", err)
		}
		return DefaultClampConfig()
	}

	if cfg.Defaults == nil {
		cfg.Defaults = DefaultClampConfig().Defaults
	}
	if cfg.Cities == nil {
		cfg.Cities = map[string]map[domainlistings.RentalTermType]ClampRange{}
	}

	normalizedCities := make(map[string]map[domainlistings.RentalTermType]ClampRange, len(cfg.Cities))
	for city, terms := range cfg.Cities {
		key := NormalizeCity(city)
		if key == "" {
			continue
		}
		if terms == nil {
			continue
		}
		normalizedCities[key] = terms
	}
	cfg.Cities = normalizedCities
	return cfg
}

func NormalizeCity(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	switch strings.ToLower(trimmed) {
	case "moscow", "москва":
		return "Москва"
	case "krasnodar", "краснодар":
		return "Краснодар"
	default:
		return trimmed
	}
}

func applyClamps(amount int64, cfg ClampConfig, city string, term domainlistings.RentalTermType) (final int64, min int64, max int64, clamped bool) {
	final = amount
	term = normalizeClampTerm(term)
	city = NormalizeCity(city)

	rng, ok := cfg.Cities[city][term]
	if !ok {
		rng, ok = cfg.Defaults[term]
	}
	if !ok {
		return final, 0, 0, false
	}

	min = rng.MinRub
	max = rng.MaxRub

	if min > 0 && final < min {
		final = min
		clamped = true
	}
	if max > 0 && final > max {
		final = max
		clamped = true
	}
	return final, min, max, clamped
}

func normalizeClampTerm(term domainlistings.RentalTermType) domainlistings.RentalTermType {
	switch term {
	case domainlistings.RentalTermShort, domainlistings.RentalTermLong:
		return term
	default:
		return domainlistings.RentalTermLong
	}
}

