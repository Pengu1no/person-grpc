package http_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"person-grpc/internal/domain"
	"person-grpc/internal/ports"
	"sync"
	"time"
)

type agifyResult struct {
	Name  string `json:"name"`
	Age   uint32 `json:"age"`
	Count uint32 `json:"count"`
}

type genderizeResult struct {
	Name        string  `json:"name"`
	Gender      string  `json:"gender"`
	Count       uint32  `json:"count"`
	Probability float32 `json:"probability"`
}

type countryData struct {
	CountryID   string  `json:"country_id"`
	Probability float32 `json:"probability"`
}

type nationalizeResult struct {
	Name    string `json:"name"`
	Count   uint32 `json:"count"`
	Country []countryData
}

type AgifyClient struct {
	client *http.Client
}

func NewAgifyClient(timeout time.Duration) *AgifyClient {
	return &AgifyClient{client: &http.Client{Timeout: timeout}}
}

func (c *AgifyClient) GetAge(ctx context.Context, fullName string) (uint32, error) {
	q := url.Values{}
	q.Add("name", fullName)
	apiUrl := "https://api.agify.io?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("agify http status code: %d", resp.StatusCode)
	}

	var res agifyResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return 0, err
	}
	return res.Age, nil
}

type GenderizeClient struct {
	client *http.Client
}

func NewGenderizeClient(timeout time.Duration) *GenderizeClient {
	return &GenderizeClient{client: &http.Client{Timeout: timeout}}
}

func (c *GenderizeClient) GetGender(ctx context.Context, fullName string) (domain.Gender, error) {
	q := url.Values{}
	q.Add("name", fullName)
	apiUrl := "https://api.genderize.io?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return domain.GenderUnspecified, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return domain.GenderUnspecified, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.GenderUnspecified, fmt.Errorf("genderize http status code: %d", resp.StatusCode)
	}

	var res genderizeResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return domain.GenderUnspecified, err
	}

	switch res.Gender {
	case "male":
		return domain.GenderMale, nil
	case "female":
		return domain.GenderFemale, nil
	default:
		return domain.GenderUnspecified, nil
	}
}

type NationalizeClient struct {
	client *http.Client
}

func NewNationalizeClient(timeout time.Duration) *NationalizeClient {
	return &NationalizeClient{client: &http.Client{Timeout: timeout}}
}

func (c *NationalizeClient) GetNationality(ctx context.Context, fullName string) (string, error) {
	q := url.Values{}
	q.Add("name", fullName)
	apiUrl := "https://api.nationalize.io?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiUrl, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nationalize http status code: %d", resp.StatusCode)
	}

	var res nationalizeResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Country) == 0 {
		return "", fmt.Errorf("nationalize unable to determine")
	}

	return res.Country[0].CountryID, nil
}

type ParallelEnricher struct {
	ageProvider         ports.AgeProvider
	genderProvider      ports.GenderProvider
	nationalityProvider ports.NationalityProvider

	semaphore chan struct{}
}

func NewParallelEnricher(age ports.AgeProvider, gender ports.GenderProvider, nationality ports.NationalityProvider, maxConcurrent int) *ParallelEnricher {
	return &ParallelEnricher{
		ageProvider:         age,
		genderProvider:      gender,
		nationalityProvider: nationality,
		semaphore:           make(chan struct{}, maxConcurrent),
	}
}

func (pe *ParallelEnricher) withRetry(ctx context.Context, operation func(ctx context.Context) error) error {
	maxAttempts := 3
	backoff := 100 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			slog.Debug("ParallelEnricher.withRetry: context deadline exceeded")
			return ctx.Err()
		}

		select {
		case pe.semaphore <- struct{}{}:
			slog.Debug("ParallelEnricher.withRetry: semaphore acquired")
		case <-ctx.Done():
			slog.Debug("ParallelEnricher.withRetry: context deadline exceeded")
			return ctx.Err()
		}

		err := operation(ctx)
		slog.Debug("ParallelEnricher.withRetry: enriching operation")

		<-pe.semaphore
		slog.Debug("ParallelEnricher.withRetry: semaphore released")

		if err == nil {
			slog.Debug("ParallelEnricher.withRetry: enrichment completed")
			return nil
		}

		if attempt == maxAttempts {
			slog.Debug("ParallelEnricher.withRetry: enrichment failed (max attempts exceeded)", slog.Uint64("attempts", uint64(attempt)))
			return err
		}

		select {
		case <-time.After(backoff * time.Duration(attempt)):
			slog.Debug("ParallelEnricher.withRetry: backoff exceeded")
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("unexpected end of retry loop")
}

func (pe *ParallelEnricher) Enrich(ctx context.Context, fullName string) (uint32, domain.Gender, string, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	var age uint32
	gender := domain.GenderUnspecified
	var nationality string

	wg.Add(3)

	go func() {
		defer wg.Done()
		err := pe.withRetry(ctx, func(ctx context.Context) error {
			var e error
			age, e = pe.ageProvider.GetAge(ctx, fullName)
			return e
		})
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to enrich age: %w", err))
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		err := pe.withRetry(ctx, func(ctx context.Context) error {
			var e error
			gender, e = pe.genderProvider.GetGender(ctx, fullName)
			return e
		})
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to enrich gender: %w", err))
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		err := pe.withRetry(ctx, func(ctx context.Context) error {
			var e error
			nationality, e = pe.nationalityProvider.GetNationality(ctx, fullName)
			return e
		})
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to enrich nationality: %w", err))
			mu.Unlock()
		}
	}()

	wg.Wait()

	var finalErr error
	if len(errs) > 0 {
		finalErr = errors.Join(errs...)
	}

	return age, gender, nationality, finalErr
}
