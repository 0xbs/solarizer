package solarweb

import "encoding/json"

type CompareData struct {
	IsOnline          bool    `json:"IsOnline"`
	AllOnline         bool    `json:"AllOnline"`
	PowerGrid         float64 `json:"P_Grid"` // Watts from Grid to Inverter
	PowerLoad         float64 `json:"P_Load"` // Watts from House to Inverter
	PowerPV           float64 `json:"P_PV"`   // Watts from Cells to Inverter
	PowerBattery      float64 `json:"P_Batt"` // Watts from Battery to Inverter
	BatteryPercentage float64 `json:"SOC"`
	BatteryMode       float64 `json:"BatMode"`
	//OhmPilots         []any   `json:"Ohmpilots"`
	//WattPilots        []any   `json:"Wattpilots"`
	//Consumers         []any   `json:"Consumers"`
	//Generators        []any   `json:"Generators"`
}

type EarningAndSavings struct {
	Data struct {
		Earnings struct {
			IsoCurrency string `json:"IsoCurrency"`
			Total       string `json:"Total"`
			Month       string `json:"Month"`
			Year        string `json:"Year"`
			Today       string `json:"Today"`
			TotalLabel  string `json:"TotalLabel"`
			MonthLabel  string `json:"MonthLabel"`
			YearLabel   string `json:"YearLabel"`
			TodayLabel  string `json:"TodayLabel"`
		} `json:"Earnings"`
		TotalCo2Savings struct {
			DistanceUnit  string `json:"DistanceUnit"`
			EmissionValue string `json:"EmissionValue"`
			EmissionUnit  string `json:"EmissionUnit"`
			Trees         string `json:"Trees"`
			DistanceValue string `json:"DistanceValue"`
		} `json:"TotalCo2Savings"`
	} `json:"data"`
}

// WidgetChart is only a very reduced structure of the actual data returned
type WidgetChart struct {
	HasMeter bool   `json:"hasMeter"`
	ToGrid   string `json:"toGrid"`
	FromGrid string `json:"fromGrid"`
	Chart    struct {
		Series []struct {
			Type string            `json:"type"`
			Name string            `json:"name"`
			Data []json.RawMessage `json:"data"` // actual structure depends on type/name
		} `json:"series"`
	} `json:"chart"`
}

type AreaSplineData [][]float64
type BubbleData struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type UnreadMessageCount struct {
	Data struct {
		UnreadServiceMessages int `json:"UnreadServiceMessages"`
		UnreadNews            int `json:"UnreadNews"`
		UnreadSystemMessages  int `json:"UnreadSystemMessages"`
		PendingInvitations    int `json:"PendingInvitations"`
		Sum                   int `json:"Sum"`
	} `json:"data"`
}
