package influx

import (
	"bytes"
	"context"
	"github.com/charmbracelet/log"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxdb2api "github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	lp "github.com/influxdata/line-protocol"
	"solarizer/solarweb"
	"strconv"
	"strings"
	"time"
)

const (
	fastInterval = 15 * time.Second
	slowInterval = 5 * time.Minute
)

type DBConfig struct {
	Url    string
	Token  string
	Org    string
	Bucket string
}

type Importer struct {
	influxClient   influxdb2.Client
	influxWriteAPI influxdb2api.WriteAPI
	solarWebClient *solarweb.SolarWeb
}

func NewImporter(dbConfig DBConfig, solarWebClient *solarweb.SolarWeb) *Importer {
	client := influxdb2.NewClientWithOptions(dbConfig.Url, dbConfig.Token, influxdb2.DefaultOptions())
	writeAPI := client.WriteAPI(dbConfig.Org, dbConfig.Bucket)
	return &Importer{
		influxClient:   client,
		influxWriteAPI: writeAPI,
		solarWebClient: solarWebClient,
	}
}

func (i *Importer) RunImportLoop(ctx context.Context) {
	fastTicker := time.NewTicker(fastInterval)
	slowTicker := time.NewTicker(slowInterval)
	for {
		select {
		case <-fastTicker.C:
			i.RunFastImport()
		case <-slowTicker.C:
			i.RunSlowImport()
		case <-ctx.Done():
			return
		}
	}
}

func (i *Importer) RunFastImport() {
	log.Debug("Running fast import")
	go i.writePowerData()
}

func (i *Importer) RunSlowImport() {
	log.Debug("Running slow import")
	go i.writeEarningsData()
	go i.writeBalanceData()
}

func (i *Importer) writePowerData() {
	data, err := i.solarWebClient.GetCompareData()
	if err != nil {
		log.Error("Error fetching power data", "err", err)
		return
	}
	point := influxdb2.NewPointWithMeasurement("power").
		AddTag("is_online", strconv.FormatBool(data.IsOnline)).
		AddTag("all_online", strconv.FormatBool(data.AllOnline)).
		AddField("power_pv", data.PowerPV).
		AddField("power_grid", data.PowerGrid).
		AddField("power_load", data.PowerLoad).
		AddField("power_battery", data.PowerBattery).
		AddField("battery_percentage", data.BatteryPercentage).
		AddField("battery_mode", data.BatteryMode).
		SetTime(time.Now())
	logPoint(point)
	i.influxWriteAPI.WritePoint(point)
}

func (i *Importer) writeEarningsData() {
	data, err := i.solarWebClient.GetEarningsAndSavings()
	if err != nil {
		log.Error("Error fetching power data", "err", err)
		return
	}

	earnings := influxdb2.NewPointWithMeasurement("earnings").
		AddTag("currency", data.Data.Earnings.IsoCurrency).
		AddTag("year_name", data.Data.Earnings.YearLabel).
		AddTag("month_name", data.Data.Earnings.MonthLabel).
		AddField("total", parseLocalizedFloat(data.Data.Earnings.Total)).
		AddField("year", parseLocalizedFloat(data.Data.Earnings.Year)).
		AddField("month", parseLocalizedFloat(data.Data.Earnings.Month)).
		AddField("day", parseLocalizedFloat(data.Data.Earnings.Today)).
		SetTime(time.Now())
	logPoint(earnings)
	i.influxWriteAPI.WritePoint(earnings)

	savings := influxdb2.NewPointWithMeasurement("co2savings").
		AddTag("distance_unit", data.Data.TotalCo2Savings.DistanceUnit).
		AddTag("emission_unit", data.Data.TotalCo2Savings.EmissionUnit).
		AddField("distance", parseLocalizedFloat(data.Data.TotalCo2Savings.DistanceValue)).
		AddField("emission", parseLocalizedFloat(data.Data.TotalCo2Savings.EmissionValue)).
		AddField("trees", parseLocalizedFloat(data.Data.TotalCo2Savings.Trees)).
		SetTime(time.Now())
	logPoint(savings)
	i.influxWriteAPI.WritePoint(savings)
}

func (i *Importer) writeBalanceData() {
	data, err := i.solarWebClient.GetWidgetChart()
	if err != nil {
		log.Error("Error fetching power data", "err", err)
		return
	}

	balance := influxdb2.NewPointWithMeasurement("balance").
		AddTag("has_meter", strconv.FormatBool(data.HasMeter)).
		AddField("kwh_to_grid_today", parseLocalizedFloatWithUnit(data.ToGrid)).
		AddField("kwh_from_grid_today", parseLocalizedFloatWithUnit(data.FromGrid)).
		SetTime(time.Now())
	logPoint(balance)
	i.influxWriteAPI.WritePoint(balance)
}

// parseLocalizedFloatWithUnit converts strings like "12,4 kWh" into float64 12.4
func parseLocalizedFloatWithUnit(value string) float64 {
	parts := strings.Split(value, " ")
	if len(parts) > 0 {
		return parseLocalizedFloat(parts[0])
	}
	return 0.0
}

// parseLocalizedFloat converts strings like "1.012,4" into float64 1012.4
func parseLocalizedFloat(value string) float64 {
	val := value
	val = strings.ReplaceAll(val, ".", "")     // remove thousands separators if present
	val = strings.Replace(val, ",", ".", 1)    // comma to dot
	number, err := strconv.ParseFloat(val, 64) // parse as float
	if err != nil {
		return 0.0
	}
	return number
}

func pointToLineProtocol(point *write.Point) (string, error) {
	var buf bytes.Buffer
	encoder := lp.NewEncoder(&buf)
	_, err := encoder.Encode(point)
	return buf.String(), err
}

func logPoint(point *write.Point) {
	lineProtocol, err := pointToLineProtocol(point)
	if err != nil {
		log.Warn("unable to convert point to line protocol", "err", err)
	} else {
		log.Debug("point", "lp", strings.TrimSpace(lineProtocol))
	}
}
