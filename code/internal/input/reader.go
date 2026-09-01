package input

import (
	"fmt"
	"math"
	"strings"

	"github.com/xuri/excelize/v2"

	"waste-routing/internal/models"
	"waste-routing/internal/utils"
)

const (
	wasteDensityTonsPerM3     = 0.2
	warmPeriodCoefficient     = 6.258
	coldPeriodCoefficient     = 5.7
	capacityThousandTonsToTon = 1000
)

// ReadWasteSources reads and normalizes waste accumulation points from an XLSX sheet.
func ReadWasteSources(path string, sheet string) ([]models.WasteSource, error) {
	rows, err := readRows(path, sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: sheet %q is empty", path, sheet)
	}

	headers := makeHeaderIndex(rows[0])
	required := []string{
		"id площадки",
		"адрес",
		"долгота",
		"широта",
		"объем",
		"количество",
	}
	if err := requireHeaders(path, sheet, headers, required); err != nil {
		return nil, err
	}

	sources := make([]models.WasteSource, 0, len(rows)-1)
	for rowIndex, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}

		volume := parseFloat(cell(row, headers, "объем"))
		count := parseInt(cell(row, headers, "количество"))
		warmPickups := parseFloat(cell(row, headers, "вывозов в теплое время"))
		coldPickups := parseFloat(cell(row, headers, "вывозов в холодное время"))
		source := models.WasteSource{
			ID:                cell(row, headers, "id площадки"),
			ServiceZone:       cell(row, headers, "сервисная зона"),
			Address:           cell(row, headers, "адрес"),
			District:          cell(row, headers, "район"),
			WasteType:         cell(row, headers, "тип мусора"),
			ContainerType:     cell(row, headers, "тип контейнера"),
			ContainerVolumeM3: volume,
			ContainerCount:    count,
			WasteAmount:       annualWasteTons(volume, count, warmPickups, coldPickups),
			PickupsWarmPeriod: warmPickups,
			PickupsColdPeriod: coldPickups,
			Latitude:          parseFloat(cell(row, headers, "широта")),
			Longitude:         parseFloat(cell(row, headers, "долгота")),
		}
		if source.ID == "" {
			source.ID = fmt.Sprintf("row-%d", rowIndex+2)
		}
		sources = append(sources, source)
	}

	return sources, nil
}

// ReadTransferStations reads and normalizes transfer stations from an XLSX sheet.
func ReadTransferStations(path string, sheet string) ([]models.TransferStation, error) {
	rows, err := readRows(path, sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: sheet %q is empty", path, sheet)
	}

	headers := makeHeaderIndex(rows[0])
	required := []string{"id", "наименование мпс", "мощность", "широта", "долгота"}
	if err := requireHeaders(path, sheet, headers, required); err != nil {
		return nil, err
	}

	stations := make([]models.TransferStation, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}
		stations = append(stations, models.TransferStation{
			ID:        cell(row, headers, "id"),
			Name:      cell(row, headers, "наименование мпс"),
			Capacity:  capacityTonsPerYear(cell(row, headers, "мощность")),
			Latitude:  parseFloat(cell(row, headers, "широта")),
			Longitude: parseFloat(cell(row, headers, "долгота")),
		})
	}

	return stations, nil
}

// ReadPlants reads and normalizes plants from an XLSX sheet.
func ReadPlants(path string, sheet string) ([]models.Plant, error) {
	rows, err := readRows(path, sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: sheet %q is empty", path, sheet)
	}

	headers := makeHeaderIndex(rows[0])
	required := []string{"id", "мощность", "широта", "долгота"}
	if err := requireHeaders(path, sheet, headers, required); err != nil {
		return nil, err
	}
	if !hasAnyHeader(headers, "наименование", "наименование мпс") {
		return nil, fmt.Errorf("%s: sheet %q missing required plant name column: Наименование or Наименование МПС", path, sheet)
	}

	plants := make([]models.Plant, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if isEmptyRow(row) {
			continue
		}
		plants = append(plants, models.Plant{
			ID:        cell(row, headers, "id"),
			Name:      cell(row, headers, "наименование", "наименование мпс"),
			Capacity:  capacityTonsPerYear(cell(row, headers, "мощность")),
			Latitude:  parseFloat(cell(row, headers, "широта")),
			Longitude: parseFloat(cell(row, headers, "долгота")),
		})
	}

	return plants, nil
}

func readRows(path string, sheet string) ([][]string, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	if sheet == "" {
		sheet = file.GetSheetName(0)
	}
	if sheet == "" {
		return nil, fmt.Errorf("%s: no sheets found", path)
	}

	rows, err := file.GetRows(sheet, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func makeHeaderIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, name := range header {
		normalized := utils.NormalizeHeader(name)
		if normalized == "" {
			continue
		}
		index[normalized] = i
	}
	return index
}

func requireHeaders(path string, sheet string, headers map[string]int, required []string) error {
	var missing []string
	for _, header := range required {
		if _, ok := headers[utils.NormalizeHeader(header)]; !ok {
			missing = append(missing, header)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: sheet %q missing required columns: %s", path, sheet, strings.Join(missing, ", "))
	}
	return nil
}

func hasAnyHeader(headers map[string]int, names ...string) bool {
	for _, name := range names {
		if _, ok := headers[utils.NormalizeHeader(name)]; ok {
			return true
		}
	}
	return false
}

func cell(row []string, headers map[string]int, names ...string) string {
	for _, name := range names {
		index, ok := headers[utils.NormalizeHeader(name)]
		if !ok || index >= len(row) {
			continue
		}
		return strings.TrimSpace(row[index])
	}
	return ""
}

func isEmptyRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func parseFloat(value string) float64 {
	parsed, err := utils.ParseFlexibleFloat(value)
	if err != nil {
		return math.NaN()
	}
	return parsed
}

func parseInt(value string) int {
	parsed := parseFloat(value)
	if math.IsNaN(parsed) {
		return 0
	}
	return int(math.Round(parsed))
}

func annualWasteTons(containerVolumeM3 float64, containerCount int, warmPickupsPerMonth float64, coldPickupsPerMonth float64) float64 {
	if math.IsNaN(containerVolumeM3) || containerCount <= 0 {
		return math.NaN()
	}
	if math.IsNaN(warmPickupsPerMonth) && math.IsNaN(coldPickupsPerMonth) {
		return math.NaN()
	}
	if math.IsNaN(warmPickupsPerMonth) {
		warmPickupsPerMonth = coldPickupsPerMonth
	}
	if math.IsNaN(coldPickupsPerMonth) {
		coldPickupsPerMonth = warmPickupsPerMonth
	}

	annualPickups := warmPickupsPerMonth*warmPeriodCoefficient + coldPickupsPerMonth*coldPeriodCoefficient
	annualVolumeM3 := containerVolumeM3 * float64(containerCount) * annualPickups
	return annualVolumeM3 * wasteDensityTonsPerM3
}

func capacityTonsPerYear(value string) float64 {
	capacity := parseFloat(value)
	if math.IsNaN(capacity) {
		return math.NaN()
	}
	return capacity * capacityThousandTonsToTon
}
