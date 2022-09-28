package plane

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	transport "github.com/ddnWizzie/transport"
	"github.com/icholy/utm"
	geo "github.com/kellydunn/golang-geo"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/mat"
)

var (
	// Prometheus
	Distance = flag.String("Distance", "30", "Para calcular el modulo se le resta esta distancia")
	Single   = flag.String("Single", "1", "Metrics path")
)

type StatePlane struct {
	values sync.Map
	Input  *transport.KafkaConsumerState
}

func (s *StatePlane) PlaneService() {
	log.Info("Creating Plane")
	topic := []string{*transport.KafkaTopicPlaneIn}
	wg := &sync.WaitGroup{}
	ctx, _ := context.WithCancel(context.Background())
	uuid := uuid.NewV4()
	consumerGroup := fmt.Sprintf("%s_%s", *transport.ConsumerGroup, uuid)

	consumer, err := transport.StartConsumerFromTopic(topic, consumerGroup, "oldest")

	if err != nil {
		log.Fatal("Could not start Consumer from topic %v", err)
	}
	fmt.Printf("ConsumerGroup %s for topic %v", consumerGroup, topic)

	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			// `Consume` should be called inside an infinite loop, when a
			// server-side rebalance happens, the consumer session will need to be
			// recreated to get the new claims
			if err := consumer.Client.Consume(ctx, consumer.Topics, &consumer.Consumer); err != nil {
				log.Panicf("Error from consumer in processor %s: %v", err)
			}
			// check if context was cancelled, signaling that the consumer should stop
			if ctx.Err() != nil {
				return
			}
			consumer.Consumer.Ready = make(chan bool)

		}
	}(wg)
	fmt.Println(consumer.Topics)
	<-consumer.Consumer.Ready // Await till the consumer has been set up
	s.Input = consumer

	wg.Add(1)
	// go func(wg *sync.WaitGroup) {
	// 	defer wg.Done()
	for {
		// begin to process incoming messages one by one
		msg := <-consumer.Consumer.Msgs
		// fmt.Println(string(msg))

		s.CreatePolygon(msg)
		// s.AddPolygon(msg)
		time.Sleep(10 * time.Second)
		// s.LatlngToPointMatrices(msg)
		width := 35.0
		height := 21.0

		x := 261.0
		y := 208.0

		s.TranslatePercentToLatlng(x, y, width, height, msg)

	}
	// }(s.Wg)
}

func (s *StatePlane) CreatePolygon(msg []byte) error {
	log.Infof("Creating Polygon")

	var points []*geo.Point
	var key string

	Out := ZonePolygon{}
	znePlane := ZonePlane{}
	zneLayouts := ZoneLayouts{}
	var m map[string]interface{}
	json.Unmarshal(msg, &m)
	// var outputJSON []byte

	messag := InPlane{}

	err := json.Unmarshal(msg, &messag)
	if err != nil {
		return err
	}

	zoneArray := messag.Zones
	layoutsArray := messag.Layouts
	idPlane := messag.Id
	namePlane := messag.Identifier

	if idPlane == nil {
		err = errors.New("Can not find id of plane in msg")
		return err
	}
	key = *idPlane
	Out.PlaneId = *idPlane
	Out.PlaneName = *namePlane
	log.Warn("Can not decode key of message: %v", messag)

	if len(zoneArray) > 0 {
		for _, zone := range zoneArray {
			geoCord := zone.GeoCoordinates
			idZone := zone.Id
			zName := zone.Identifier
			for _, geoc := range geoCord {
				lat := geoc.Latitude
				long := geoc.Longitude
				xy := geo.NewPoint(*lat, *long)
				points = append(points, xy)
			}
			poly := geo.NewPolygon(points)

			if poly.IsClosed() {
				znePlane.ZoneIdZP = *idZone
				znePlane.ZoneNameZP = *zName
				znePlane.Poly = poly
				Out.ZonePlane = append(Out.ZonePlane, znePlane)

				// key := *idPlane
				fmt.Println("Out.PlaneId", Out.PlaneId)
				fmt.Println("Out.PlaneName", Out.PlaneName)
				fmt.Println("Out.ZonePlane", Out.ZonePlane)
				fmt.Println("key", key)
				// s.AddElement(key, Out)
			}

			points = nil
		}
	}

	// loop by all layouts zones
	if len(layoutsArray) > 0 {
		for _, lyots := range layoutsArray {
			zones := lyots.Zones
			idlyots := lyots.Id
			namelyots := lyots.Name
			for _, zne := range zones {
				zneName := zne.Identifier
				geoCord := zne.GeoCoordinates
				for _, geoc := range geoCord {
					lat := geoc.Latitude
					long := geoc.Longitude
					xy := geo.NewPoint(*lat, *long)
					points = append(points, xy)
				}
				poly := geo.NewPolygon(points)

				if poly.IsClosed() {
					zneLayouts.LayoutsId = *idlyots
					zneLayouts.LayoutsName = *namelyots
					zneLayouts.Poly = poly
					zneLayouts.ZoneNameLyots = *zneName
					Out.ZoneLayouts = append(Out.ZoneLayouts, zneLayouts)
					fmt.Println("Out.ZoneLayouts", Out.ZoneLayouts)
					// s.values.Store(key, Out)

				}
				points = nil

			}
		}
	}
	if key != "" {
		s.AddElement(key, Out)
	}

	messag.Zones = zoneArray
	fmt.Println("maps", s.values)

	return err
}

func (s *StatePlane) AddElement(key string, msg ZonePolygon) error {

	_, ok := s.values.Load(key)

	if ok {
		// if the key already exists I overwrite it
		s.values.Delete(key)
		s.values.Store(key, msg)
		fmt.Println(" Entro por aqui 2 ")

		fmt.Println(" Overwrite elements ")
		log.Info(" key aaaa", key)
		// }
	} else {
		s.values.Store(key, msg)
		fmt.Println(" Entro por aqui 1 ")
		fmt.Println(" Add new elements ")
		log.Info(" key", key)
		// TODO add metrics
	}

	return nil
}

func (s *StatePlane) IsPointInsidePolygon(latitude string, longitude string) ([]byte, error) {
	log.Infof("Find Point inside a Plane algorithm")
	log.Infof("*****************************")
	var outputJSON []byte
	var arrZones []string
	m := make(map[string]interface{}, 1000)
	var arrZonesLy []string
	json.Unmarshal(outputJSON, &m)

	// convert string to float
	lat, err := strconv.ParseFloat(latitude, 64)
	if err != nil {
		log.Error("Could not convert latitude", err)
	}
	long, err := strconv.ParseFloat(longitude, 64)
	if err != nil {
		log.Error("Could not convert longitude", err)
	}

	m["latitude"] = lat
	m["longitude"] = long
	// make point type of my librery
	p := geo.NewPoint(lat, long)

	// Read the map with all planes
	s.values.Range(func(key, value any) bool {
		zoneA := value.(ZonePolygon).ZonePlane
		lyotsA := value.(ZonePolygon).ZoneLayouts
		for _, zone := range zoneA {
			ok := zone.Poly.Contains(p)
			if ok {

				arrZones = append(arrZones, zone.ZoneNameZP)
				// m["zones"] = append(arrZones, zone.ZoneNameZP)

				log.Infof("The point belongs to the plane %s and the area is  %s", value.(ZonePolygon).PlaneId, zone.ZoneNameZP)
			} else {
				log.Infof("The point does not belong to this plane ")
			}

		}
		m["zone_plane"] = (strings.Join(arrZones, ", "))
		// arrZones = nil

		for _, zoneLy := range lyotsA {
			ok := zoneLy.Poly.Contains(p)
			if ok {
				m["layouts"] = zoneLy.LayoutsName

				arrZonesLy = append(arrZonesLy, zoneLy.ZoneNameLyots)
				// m["Zones"] = append(arrZonesLy, zoneLy.ZoneNameLyots)

				log.Infof("The point belongs to the layout %s and the area is  %s", zoneLy.LayoutsName, zoneLy.ZoneNameLyots)
			} else {
				log.Infof("The point does not belong to this plane ")
			}

		}
		// TODO si me piden ocultar los campos si esta vacio el arreglo de los layouts

		m["zone_layouts"] = (strings.Join(arrZonesLy, ", "))
		m["name_plane"] = value.(ZonePolygon).PlaneName

		return true

	})
	outputJSON, _ = json.Marshal(m)
	a, err := s.SocialDistance(outputJSON)
	if err != nil {
		return nil, err
	}

	// TODO llamar a este metodo desde aqui y no desde el main
	return a, nil
}

func (s *StatePlane) SocialDistance(msg []byte) ([]byte, error) {

	// log.Infof("In SocialDistance algorithm")
	// fmt.Println("message in SD", string(msg))

	var m map[string]interface{}
	var outputJSON []byte
	json.Unmarshal(msg, &m)
	lat, ok := m["latitude"]
	if !ok {
		log.Info("Can't find latitude in msg %s", string(msg))
		return msg, nil
	}
	long, ok := m["longitude"]

	if !ok {
		log.Info("Can't find longitude in msg %s", string(msg))
		return msg, nil
	}

	easting, northing, _ := utm.ToUTM(lat.(float64), long.(float64))

	// fmt.Printf("Easting: %f\n", easting)
	// fmt.Printf("Northing: %f\n", northing)

	// Get the module
	if sv, err := strconv.Atoi(*Distance); err == nil {
		coord_e := int(math.Round(easting) - math.Mod(math.Round(easting), float64(sv)))
		// fmt.Printf("rounded easting: %v\n", coord_e)
		coord_n := int(math.Round(northing) - math.Mod(math.Round(northing), float64(sv)))
		// fmt.Printf("rounded northing: %v\n", coord_n)

		// Concatenating rounded coordinates
		cell := strconv.Itoa(coord_e) + "_" + strconv.Itoa(coord_n)
		// fmt.Printf("Concatenating rounded coordinates easting_northing: %v\n", cell)

		// converting geographic coordinates UTM to WSG84
		zone, _ := utm.LookupSRID(32631)
		cell_latitude, cell_longitude := zone.ToLatLon(float64(coord_e), float64(coord_n))

		// Concatenating coordinates
		// celldim_coordinates_latlong := strconv.FormatFloat(cell_latitude, 'E', -1, 64) + "," + strconv.FormatFloat(cell_longitude, 'E', -1, 64)
		cell_coordinates_latlong := fmt.Sprint(cell_latitude) + "," + fmt.Sprint(cell_longitude)
		// fmt.Printf("Concatenating float coordinates: %v\n", celldim_coordinates_latlong)

		latf := fmt.Sprintf("%.6f", lat.(float64))
		longf := fmt.Sprintf("%.6f", long.(float64))
		a, _ := strconv.ParseFloat(latf, 64)
		b, _ := strconv.ParseFloat(longf, 64)
		coordinates_latlong := fmt.Sprint(a) + "," + fmt.Sprint(b)

		m["latitude"] = a
		m["longitude"] = b
		m["cell_latitude"] = cell_latitude
		m["cell_longitude"] = cell_longitude
		m["cell"] = cell
		m["coordinates_latlong"] = coordinates_latlong

		// Add celldim_coordinates if Single is defined
		s, err := strconv.Atoi(*Single)
		if err == nil {
			// fmt.Printf("%T \n %v", s, s)
			if int(s) == 1 {
				m["cell_coordinates_latlong"] = cell_coordinates_latlong
			}
		}

		outputJSON, _ = json.Marshal(m)

	}

	return outputJSON, nil
}

/**
 *
 * Translate points from x and y utm system (percentages) to latitude and longitude.
 *
 * @param {number} x
 * @param {number} y
 * @param {number} width image
 * @param {number} hight image
 * @param {[]byte } gpsPoints
 * @returns {float64} latitude and longitude entries
 */
func (s *StatePlane) TranslatePercentToLatlng(pointX, pointY, width, hight float64, msg []byte) (lat, long float64, err error) {

	if width == 0 || hight == 0 {
		latitude := 0.0
		longitude := 0.0
		log.Warn("the width or height of the image is 0")
		return latitude, longitude, err

	}

	m_trans, m_offset, err := s.LatlngToPointMatrices(msg)
	if err != nil {
		// Returns the start point, or null if you do not want it to be showed
		latitude := 0.0
		longitude := 0.0
		log.Warn("error in method LatlngToPointMatrices: ", err)
		return latitude, longitude, err
	} else {
		latitude, longitude, err := s.PercentToLatlng(pointX, pointY, m_trans, m_offset)
		if err != nil {
			latitude := 0.0
			longitude := 0.0
			log.Warn("error in method PercentToLatlng: ", err)
			return latitude, longitude, err
		} else {
			return latitude, longitude, nil
		}
	}

}

/**
 * Obtain the matrices for conversions between latitude-longitude to x-y.
 *
 * @param {msg []byte} the plane
 * @returns {Object} transformation and offset matrices { m_trans, m_offset }
 */
func (s *StatePlane) LatlngToPointMatrices(msg []byte) (*mat.Dense, *mat.VecDense, error) {
	var m_trans *mat.Dense
	var m_offset *mat.VecDense
	var m map[string]interface{}
	json.Unmarshal(msg, &m)
	messag := InPlane{}

	err := json.Unmarshal(msg, &messag)
	if err != nil {
		return nil, nil, err
	}

	gps_markers := messag.GeoMarkers
	log.Info(gps_markers)

	// take 3 points from Gps.Markers to form the matrix
	data_xy := []float64{}
	data_latlong := []float64{}

	for i, gpsMark := range gps_markers {
		pointX := gpsMark.Coordinate.X
		pointY := gpsMark.Coordinate.Y
		lat := gpsMark.GeoCoordinate.Latitude
		long := gpsMark.GeoCoordinate.Longitude

		data_xy = append(data_xy, *pointX, *pointY, 0, 0, 1, 0, 0, 0, *pointX, *pointY, 0, 1)
		data_latlong = append(data_latlong, *long, *lat)
		// switch i {
		// case 0:
		// 	data_xy = append(data_xy, *pointX, *pointY, 0, 0, 1, 0, 0, 0, *pointX, *pointY, 0, 1)
		// 	data_latlong = append(data_latlong, *long, *lat)

		// case 1:
		// 	data_xy = append(data_xy, *pointX, *pointY, 0, 0, 1, 0, 0, 0, *pointX, *pointY, 0, 1)
		// 	data_latlong = append(data_latlong, *long, *lat)
		// case 2:
		// 	data_xy = append(data_xy, *pointX, *pointY, 0, 0, 1, 0, 0, 0, *pointX, *pointY, 0, 1)
		// 	data_latlong = append(data_latlong, *long, *lat)
		// }

		if i >= 2 {
			break
		}

	}

	matrix := mat.NewDense(6, 6, data_xy)
	lat_lng_column := mat.NewVecDense(6, data_latlong)

	// print all matrix elements formatted matrix
	fmt.Printf("matrix :\n%v\n\n", mat.Formatted(matrix, mat.Prefix(""), mat.Excerpt(0)))
	// print all lat_lng_column elements
	fmt.Printf("lat_lng_column :\n%v\n\n", mat.Formatted(lat_lng_column, mat.Prefix(""), mat.Excerpt(0)))

	// To check if the gps markers are not valid, the determinant of a matrix is calculated
	// if the determinant is zero, the matrix has no inverse and the gpsMarkers are useless.

	det := mat.Det(matrix)
	log.Info("Determinante es: ", det)

	if det == 0 {
		m_trans = nil
		m_offset = nil
		log.Warn("the determinant of a matrix is zero, invalid gpsMarkers")
		return m_trans, m_offset, err
	}

	var sol mat.Dense

	err = sol.Inverse(matrix)
	if err != nil {
		log.Fatalf("matrix is not invertible: %v", err)
	}
	// Print the inverse using the formatter.
	fa := mat.Formatted(&sol, mat.Prefix("       "), mat.Squeeze())
	fmt.Printf("sol11111 = %.2g\n\n", fa)

	// Multiplying the inverse by the matrix of the coordinates

	var sol_final mat.VecDense

	sol_final.MulVec(&sol, lat_lng_column)
	fmt.Printf("sol mulElements :\n%v\n\n", mat.Formatted(&sol_final, mat.Prefix(""), mat.Excerpt(0)))

	// Getting the first four values ​​of the array and creating an array with them
	var mt []float64

	mt = append(mt, mat.Row(nil, 0, &sol_final)...)
	mt = append(mt, mat.Row(nil, 1, &sol_final)...)
	mt = append(mt, mat.Row(nil, 2, &sol_final)...)
	mt = append(mt, mat.Row(nil, 3, &sol_final)...)

	// Put the values inside the matrix
	m_trans = mat.NewDense(2, 2, mt)

	// If the gps markers are not valid
	dette := mat.Det(m_trans)
	log.Info("Determinante es: ", dette)

	if dette == 0 {
		m_trans = nil
		m_offset = nil
		log.Warn("the determinant of a matrix is zero, invalid gpsMarkers")
		return m_trans, m_offset, err
	}

	// var m_trasp mat.Dense

	err = m_trans.Inverse(m_trans)
	if err != nil {
		log.Fatalf("matrix is not invertible: %v", err)
	}
	// Print the inverse using the formatter.
	fm := mat.Formatted(m_trans, mat.Prefix("       "), mat.Squeeze())
	fmt.Printf("m_trasp = %.2g\n\n", fm)

	var mof []float64
	mof = append(mof, mat.Row(nil, 4, &sol_final)...)
	mof = append(mof, mat.Row(nil, 5, &sol_final)...)

	m_offset = mat.NewVecDense(2, mof)

	fmt.Printf("m_offset :\n%v\n\n", mat.Formatted(m_offset, mat.Prefix(""), mat.Excerpt(0)))

	// x := 261.0
	// y := 208.0
	// s.PercentToLatlng(x, y, m_trans, m_offset)

	return m_trans, m_offset, nil
}

/**
 * Convert point from x-y percentages reference to latitude-longitude reference
 * by a square using matrices for transformation and offset.
 *
 * @param {number} x
 * @param {number} y
 * @param {Matrix} mTrans
 * @param {Matrix} mOffset
 * @returns {float64} latitude and longitude entries
 */
func (s *StatePlane) PercentToLatlng(valX, valY float64, m_trans *mat.Dense, m_offset *mat.VecDense) (lat, long float64, err error) {
	log.Info("valX", valX)
	log.Info("valY", valY)
	log.Info("Aqui segundo metodo ******************************")
	utmX := valX // x * width;
	utmY := valY // y * height;

	// Create matrix with x and y point
	utmArrXY := []float64{}
	utmArrXY = append(utmArrXY, utmX, utmY)
	utmPoint := mat.NewVecDense(2, utmArrXY)

	// Calc the inverse of matrix.
	err = m_trans.Inverse(m_trans)
	if err != nil {
		log.Fatalf("matrix is not invertible: %v", err)
	}
	// Print the inverse using the formatter.
	fm := mat.Formatted(m_trans, mat.Prefix("       "), mat.Squeeze())
	fmt.Printf("m_trasp = %.2g\n\n", fm)

	// multiply the inverse by the vector
	var inv_xvec mat.VecDense

	inv_xvec.MulVec(m_trans, utmPoint)
	fmt.Printf("inversa por el vector :\n%v\n\n", mat.Formatted(&inv_xvec, mat.Prefix(""), mat.Excerpt(0)))

	fmt.Printf("m_offset :\n%v\n\n", mat.Formatted(m_offset, mat.Prefix(""), mat.Excerpt(0)))

	// sum of the result of the multiplication plus the m_offset
	var latlonPoint mat.VecDense
	// m_offset.AddVec(&inv_xvec, m_offset)
	latlonPoint.AddVec(&inv_xvec, m_offset)

	fmt.Printf("latlonPoint :\n%v\n\n", mat.Formatted(&latlonPoint, mat.Prefix(""), mat.Excerpt(0)))

	// Take lat and long to the matrix
	lattud := mat.Row(nil, 1, &latlonPoint)
	latitude := lattud[0]
	log.Info("latitude: ", latitude)

	longtud := mat.Row(nil, 0, &latlonPoint)
	longitude := longtud[0]
	log.Info("longitude: ", longitude)

	return latitude, longitude, nil
}

func ConvertInt(val string, base, toBase int) (string, error) {
	i, err := strconv.ParseInt(val, base, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(i, toBase), nil
}
