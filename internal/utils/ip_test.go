package utils

import (
	"sync"
	"testing"
	"time"
)

func TestPlaceholderGeoLocation(t *testing.T) {
	tests := []struct {
		name            string
		ipHash          string
		expectedCountry string
		expectedCity    string
	}{
		{"hash terminando em 0", "abc0", "BR", "São Paulo"},
		{"hash terminando em 1", "abc1", "BR", "São Paulo"},
		{"hash terminando em 3", "abc3", "BR", "São Paulo"},
		{"hash terminando em 4", "abc4", "US", "New York"},
		{"hash terminando em 5", "abc5", "US", "New York"},
		{"hash terminando em 6", "abc6", "US", "New York"},
		{"hash terminando em 7", "abc7", "PT", "Lisbon"},
		{"hash terminando em 9", "abc9", "PT", "Lisbon"},
		{"hash terminando em a", "abca", "JP", "Tokyo"},
		{"hash terminando em b", "abcb", "JP", "Tokyo"},
		{"hash terminando em c", "abcc", "JP", "Tokyo"},
		{"hash terminando em d", "abcd", "DE", "Berlin"},
		{"hash terminando em e", "abce", "DE", "Berlin"},
		{"hash terminando em f", "abcf", "DE", "Berlin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			country, city := PlaceholderGeoLocation(tt.ipHash)
			if country != tt.expectedCountry {
				t.Errorf("country: esperava %q, obteve %q", tt.expectedCountry, country)
			}
			if city != tt.expectedCity {
				t.Errorf("city: esperava %q, obteve %q", tt.expectedCity, city)
			}
		})
	}

	t.Run("hash vazio deve retornar BR/Desconhecida", func(t *testing.T) {
		country, city := PlaceholderGeoLocation("")
		if country != "BR" || city != "Desconhecida" {
			t.Errorf("esperava BR/Desconhecida, obteve %s/%s", country, city)
		}
	})
}

func TestIsUniqueAccess(t *testing.T) {
	// Reset o cache para cada teste
	antiF5Mutex.Lock()
	antiF5Cache = make(map[string]time.Time)
	antiF5Mutex.Unlock()

	t.Run("primeiro acesso deve ser único", func(t *testing.T) {
		key := "test_unique_1::fingerprint"
		if !IsUniqueAccess(key) {
			t.Error("primeiro acesso deve retornar true")
		}
	})

	t.Run("segundo acesso imediato NÃO deve ser único", func(t *testing.T) {
		key := "test_unique_2::fingerprint"
		IsUniqueAccess(key) // primeiro
		if IsUniqueAccess(key) {
			t.Error("segundo acesso imediato deve retornar false (Anti-F5)")
		}
	})

	t.Run("fingerprints diferentes devem ser independentes", func(t *testing.T) {
		a := "link_A::fp_A"
		b := "link_A::fp_B"
		IsUniqueAccess(a)
		if !IsUniqueAccess(b) {
			t.Error("fingerprint diferente deve ser considerado único")
		}
	})

	t.Run("acesso após cooldown deve ser único novamente", func(t *testing.T) {
		originalCooldown := CooldownPeriod
		CooldownPeriod = 50 * time.Millisecond // reduzir para teste
		defer func() { CooldownPeriod = originalCooldown }()

		key := "test_cooldown::fp"
		IsUniqueAccess(key) // primeiro
		time.Sleep(100 * time.Millisecond)
		if !IsUniqueAccess(key) {
			t.Error("acesso pós-cooldown deve retornar true")
		}
	})
}

func TestIsUniqueAccess_Concurrency(t *testing.T) {
	antiF5Mutex.Lock()
	antiF5Cache = make(map[string]time.Time)
	antiF5Mutex.Unlock()

	var wg sync.WaitGroup
	uniqueCount := 0
	var mu sync.Mutex

	// 50 goroutines fazendo acesso simultâneo com mesmo key
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if IsUniqueAccess("concurrent_test::same_fp") {
				mu.Lock()
				uniqueCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if uniqueCount != 1 {
		t.Errorf("esperava exatamente 1 acesso único, obteve %d", uniqueCount)
	}
}
