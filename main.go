package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type ViaCep struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Unidade     string `json:"unidade"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
}

type Coordinates struct {
	Longitude string `json:"longitude"`
	Latitude  string `json:"latitude"`
}

type Location struct {
	Type        string      `json:"type"`
	Coordinates Coordinates `json:"coordinates"`
}

type BrasilApi struct {
	Cep          string   `json:"cep"`
	State        string   `json:"state"`
	City         string   `json:"city"`
	Neighborhood *string  `json:"neighborhood"` // Pointer to handle null
	Street       *string  `json:"street"`       // Pointer to handle null
	Service      string   `json:"service"`
	Location     Location `json:"location"`
}

func main() {
	http.HandleFunc("/", FetchBothHandler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}

func ViaCepQueue(cep string, ch chan<- *ViaCep) {
	viaCep, err := FetchViaCep(cep)
	if err != nil {
		log.Printf("Error fetching ViaCep: %v", err)
		ch <- nil
		return
	}
	ch <- viaCep
}

func FetchViaCep(cep string) (*ViaCep, error) {
	req, err := http.NewRequest(http.MethodGet, "http://viacep.com.br/ws/"+cep+"/json/", nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var viaCep ViaCep
	err = json.Unmarshal(body, &viaCep)
	if err != nil {
		return nil, err
	}

	return &viaCep, nil
}

func BrasilApiQueue(cep string, ch chan<- *BrasilApi) {
	brasilApi, err := FetchBrasilApi(cep)
	if err != nil {
		log.Printf("Error fetching BrasilApi: %v", err)
		ch <- nil
		return
	}
	ch <- brasilApi
}

func FetchBrasilApi(cep string) (*BrasilApi, error) {
	req, err := http.NewRequest(http.MethodGet, "https://brasilapi.com.br/api/cep/v2/"+cep, nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var brasilApi BrasilApi
	err = json.Unmarshal(body, &brasilApi)
	if err != nil {
		return nil, err
	}

	return &brasilApi, nil
}

func printJSON(label string, v interface{}) {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("Error marshalling to JSON: %v", err)
		return
	}
	fmt.Printf("%s response:\n%s\n", label, string(jsonBytes))
}

func FetchBothHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	cep := queryParams.Get("cep")
	if cep == "" {
		http.Error(w, "Missing 'cep' query parameter", http.StatusBadRequest)
		return
	}

	// Set a timeout for the context
	timeout := 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	viaCepCh := make(chan *ViaCep)
	brasilApiCh := make(chan *BrasilApi)

	go ViaCepQueue(cep, viaCepCh)
	go BrasilApiQueue(cep, brasilApiCh)

	var viaCep *ViaCep
	var brasilApi *BrasilApi

	// select the faster response or timeout
	select {
	case viaCep = <-viaCepCh:
		printJSON("ViaCep", viaCep)
	case brasilApi = <-brasilApiCh:
		printJSON("BrasilApi", brasilApi)
	case <-ctx.Done():
		log.Printf("Timeout reached while fetching data")
		http.Error(w, "Timeout reached", http.StatusRequestTimeout)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}
