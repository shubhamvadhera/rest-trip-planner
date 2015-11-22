package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

//Debugging variables
var out io.Writer
var debugModeActivated bool

//Global variables
var routeCombsString string
var scoreMap map[locationPair]score

//PostRequest struct to handle POST data
type PostRequest struct {
	StartingFromLocationID string   `json:"starting_from_location_id"`
	LocationIDs            []string `json:"location_ids"`
}

//PostGetResponse struct
type PostGetResponse struct {
	ID                     bson.ObjectId `json:"id" bson:"_id"`
	Status                 string        `json:"status" bson:"status"`
	StartingFromLocationID string        `json:"starting_from_location_id" bson:"starting_from_location_id"`
	BestRouteLocationIDs   []string      `json:"best_route_location_ids" bson:"best_route_location_ids"`
	TotalUberCosts         float64       `json:"total_uber_costs" bson:"total_uber_costs"`
	TotalUberDuration      float64       `json:"total_uber_duration" bson:"total_uber_duration"`
	TotalDistance          float64       `json:"total_distance" bson:"total_distance"`
}

//PutResponse struct
type PutResponse struct {
	ID                        bson.ObjectId `json:"id" bson:"_id"`
	Status                    string        `json:"status" bson:"status"`
	StartingFromLocationID    string        `json:"starting_from_location_id" bson:"starting_from_location_id"`
	NextDestinationLocationID string        `json:"next_destination_location_id" bson:"next_destination_location_id"`
	BestRouteLocationIDs      []string      `json:"best_route_location_ids" bson:"best_route_location_ids"`
	TotalUberCosts            float64       `json:"total_uber_costs" bson:"total_uber_costs"`
	TotalUberDuration         float64       `json:"total_uber_duration" bson:"total_uber_duration"`
	TotalDistance             float64       `json:"total_distance" bson:"total_distance"`
	UberWaitTimeETA           float64       `json:"uber_wait_time_eta" bson:"uber_wait_time_eta"`
}

//Point struct to hold coordinates
type Point struct {
	Lat float64 `json:"lat" bson:"lat"`
	Lng float64 `json:"lng" bson:"lng"`
}

//LocationData struct
type LocationData struct {
	ID         bson.ObjectId `json:"id" bson:"_id"`
	Name       string        `json:"name" bson:"name"`
	Address    string        `json:"address" bson:"address"`
	City       string        `json:"city" bson:"city"`
	State      string        `json:"state" bson:"state"`
	Zip        string        `json:"zip" bson:"zip"`
	Coordinate Point         `json:"coordinate" bson:"coordinate"`
}

//JSON response from Uber API
type jsonUberPrice struct {
	Prices []struct {
		DisplayName string  `json:"display_name"`
		LowEstimate float64 `json:"low_estimate"`
		Duration    float64 `json:"duration"`
		Distance    float64 `json:"distance"`
	} `json:"prices"`
}

//JSON response from Uber Request API
type jsonUberRequest struct {
	Eta float64 `json:"eta"`
}

type score struct {
	cost     float64
	duration float64
	distance float64
}

type locationPair struct {
	fromID string
	toID   string
}

//UberRideRequest to send POST request for ride request
type UberRideRequest struct {
	ProductID      string  `json:"product_id"`
	StartLatitude  float64 `json:"start_latitude"`
	StartLongitude float64 `json:"start_longitude"`
	EndLatitude    float64 `json:"end_latitude"`
	EndLongitude   float64 `json:"end_longitude"`
}

//returns mongo session for lacationdata db
func getLocationSession() (*mgo.Session, error) {
	s, err := mgo.Dial("mongodb://svadhera:cmpe273ass2@ds041934.mongolab.com:41934/locationdata")
	if err != nil {
		return s, err
	}
	return s, nil
}

//returns mongo session for tripsdata db
func getTripSession() (*mgo.Session, error) {
	s, err := mgo.Dial("mongodb://svadhera:cmpe273ass3@ds049864.mongolab.com:49864/tripsdata")
	if err != nil {
		return s, err
	}
	return s, nil
}

//returns mongo session for triprequests db
func getTripRequestSession() (*mgo.Session, error) {
	s, err := mgo.Dial("mongodb://svadhera:cmpe273ass3@ds053944.mongolab.com:53944/triprequests")
	if err != nil {
		return s, err
	}
	return s, nil
}

// CheckTrip serves the GET request
func CheckTrip(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id := p.ByName("id")
	fmt.Println("\nGET Request Received:\nID:", id)

	puresp, errCode := getTripRequestData(id)
	if errCode == "" {
		jsonOut, _ := json.Marshal(puresp)
		httpResponse(w, jsonOut, 200)
		fmt.Println("\nResponse:", string(jsonOut), "\n200 OK")
	} else {

		resp, err := getTripData(id)
		if err != nil {
			httpResponse(w, nil, 404)
			fmt.Println("Response: 404 ID Not Found")
			return
		}

		jsonOut, _ := json.Marshal(resp)
		httpResponse(w, jsonOut, 200)
		fmt.Println("\nResponse:", string(jsonOut), "\n200 OK")
	}
	fmt.Println("\nServer ready... Waiting for requests...")
}

// PlanTrip serves the POST request
func PlanTrip(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	//reset global variables
	routeCombsString = ""
	scoreMap = make(map[locationPair]score)

	var req PostRequest

	defer r.Body.Close()
	jsonIn, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpResponse(w, nil, 500)
		fmt.Println("Panic@PlanTrip.ioutil.ReadAll")
		panic(err)
	}

	fmt.Println("\nPOST Request Received:", string(jsonIn))
	json.Unmarshal([]byte(jsonIn), &req)

	bestRoute, bestScore, err := getBestRoute(req.StartingFromLocationID, req.LocationIDs)
	if err != nil {
		httpResponse(w, nil, 500)
		fmt.Println("Panic@PlanTrip.getBestRoute")
		panic(err)
	}

	resp := getPostGetResponseStruct(bestRoute, bestScore)

	fmt.Println("\n3.Saving data and generating response...")
	session, err := getTripSession()
	if err != nil {
		httpResponse(w, nil, 500)
		fmt.Println("Panic@PlanTrip.getTripSession")
		panic(err)
	}
	if err := session.DB("tripsdata").C("trips").Insert(resp); err != nil {
		httpResponse(w, nil, 500)
		fmt.Println("Panic@PlanTrip.session.DB.C.Insert")
		panic(err)
	}

	jsonOut, _ := json.Marshal(resp)
	httpResponse(w, jsonOut, 201)
	fmt.Println("\nResponse:", string(jsonOut), "\n201 Created")
	fmt.Println("\nServer ready... Waiting for requests...")
}

//RequestTrip serves the PUT request
func RequestTrip(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	id := p.ByName("id")
	fmt.Println("\nPUT Request Received:\nID:", id)

	puresp, errCode := getTripRequestData(id)
	if errCode == "id_not_exist" {
		//first PUT request
		pgresp, err := getTripData(id)
		if err != nil {
			httpResponse(w, nil, 404)
			fmt.Println("Response: 404 ID Not Found")
			return
		}

		puresp = initiateTripStruct(pgresp)

		session, err := getTripRequestSession()
		if err != nil {
			httpResponse(w, nil, 500)
			fmt.Println("Panic@RequestTrip.getTripRequestSession")
		}

		if err := session.DB("triprequests").C("requests").Insert(puresp); err != nil {
			httpResponse(w, nil, 500)
			fmt.Println("Panic@RequestTrip.session.DB.C.Insert")
		}

		jsonOut, _ := json.Marshal(puresp)
		httpResponse(w, jsonOut, 200)
		fmt.Println("\nResponse:", string(jsonOut), "\n200 OK")
	} else if errCode == "" {
		if puresp.Status == "finished" {
			jsonOut, _ := json.Marshal(puresp)
			httpResponse(w, jsonOut, 200)
			fmt.Println("\nResponse:", string(jsonOut), "\n200 OK")
		} else {
			updateTripStruct(&puresp)
			session, err := getTripRequestSession()
			if err != nil {
				httpResponse(w, nil, 500)
				fmt.Println("Panic@RequestTrip.getTripRequestSession-2")
			}

			if err := session.DB("triprequests").C("requests").UpdateId(bson.ObjectIdHex(id), puresp); err != nil {
				httpResponse(w, nil, 500)
				fmt.Println("Panic@RequestTrip.session.DB.C.Update")
			}
			jsonOut, _ := json.Marshal(puresp)
			httpResponse(w, jsonOut, 200)
			fmt.Println("\nResponse:", string(jsonOut), "\n200 OK")
		}
	} else if errCode == "invalid_id" {
		httpResponse(w, nil, 404)
	} else {
		httpResponse(w, nil, 500)
	}

	fmt.Println("\nServer ready... Waiting for requests...")
}

//Updates current trip to next location
func updateTripStruct(puresp *PutResponse) bool {

	if puresp.Status == "finished" {
		return false
	}

	if puresp.StartingFromLocationID == puresp.NextDestinationLocationID {
		puresp.Status = "finished"
		puresp.NextDestinationLocationID = ""
		puresp.UberWaitTimeETA = 0
		return true
	}

	last := len(puresp.BestRouteLocationIDs) - 1
	lastLocation := puresp.BestRouteLocationIDs[last]
	if puresp.NextDestinationLocationID == lastLocation {
		puresp.NextDestinationLocationID = puresp.StartingFromLocationID
		fromPoint, _ := getCords(lastLocation)
		toPoint, _ := getCords(puresp.NextDestinationLocationID)
		puresp.UberWaitTimeETA = getUberETA(fromPoint, toPoint)
		return true
	}

	curr := 0
	for i := 0; i <= last; i++ {
		if puresp.NextDestinationLocationID == puresp.BestRouteLocationIDs[i] {
			curr = i
			break
		}
	}
	puresp.NextDestinationLocationID = puresp.BestRouteLocationIDs[curr+1]
	fromPoint, _ := getCords(puresp.BestRouteLocationIDs[curr])
	toPoint, _ := getCords(puresp.BestRouteLocationIDs[curr+1])
	puresp.UberWaitTimeETA = getUberETA(fromPoint, toPoint)
	return true
}

//Get trips request data for Trip ID
func getTripRequestData(id string) (puresp PutResponse, errCode string) {
	if !bson.IsObjectIdHex(id) {
		return puresp, "invalid_id"
	}

	session, err := getTripRequestSession()
	if err != nil {
		return puresp, "mongo_db_session_error"
	}

	if err := session.DB("triprequests").C("requests").FindId(bson.ObjectIdHex(id)).One(&puresp); err != nil {
		return puresp, "id_not_exist"
	}
	return puresp, ""
}

//converts POST/GET response to PUT response
func initiateTripStruct(pgresp PostGetResponse) (puresp PutResponse) {
	puresp.ID = pgresp.ID
	puresp.Status = "requesting"
	puresp.StartingFromLocationID = pgresp.StartingFromLocationID
	puresp.NextDestinationLocationID = pgresp.BestRouteLocationIDs[0]
	puresp.BestRouteLocationIDs = pgresp.BestRouteLocationIDs
	puresp.TotalUberCosts = pgresp.TotalUberCosts
	puresp.TotalUberDuration = pgresp.TotalUberDuration
	puresp.TotalDistance = pgresp.TotalDistance

	fromPoint, _ := getCords(puresp.StartingFromLocationID)
	toPoint, _ := getCords(puresp.NextDestinationLocationID)

	puresp.UberWaitTimeETA = getUberETA(fromPoint, toPoint)
	return
}

//get ETA from UBER api
func getUberETA(from Point, to Point) (eta float64) {
	data := buildUberEtaJSON("https://sandbox-api.uber.com/v1/requests", from, to)
	eta = data.Eta
	return
}

//returns response struct for given route and score
func getPostGetResponseStruct(route []string, sc score) (resp PostGetResponse) {
	resp.ID = bson.NewObjectId()
	resp.Status = "planning"
	resp.StartingFromLocationID = route[0]
	resp.BestRouteLocationIDs = route[1 : len(route)-1]
	resp.TotalUberCosts = sc.cost
	resp.TotalUberDuration = sc.duration
	resp.TotalDistance = sc.distance
	return
}

//calculates the best route
func getBestRoute(startLocationID string, journeyLocationIDs []string) (bestRoute []string, bestScore score, err error) {
	if err := generateScoreMap(startLocationID, journeyLocationIDs); err != nil {
		return bestRoute, bestScore, err
	}
	permute(journeyLocationIDs, len(journeyLocationIDs), startLocationID)
	routeCombsString = routeCombsString[:len(routeCombsString)-1]
	routeCombinations := strings.Split(routeCombsString, "@")

	bestScore.cost = math.MaxFloat64
	bestScore.duration = math.MaxFloat64
	bestScore.distance = math.MaxFloat64

	for _, x := range routeCombinations {
		route := strings.Split(x, "|")
		score1 := getRouteScore(route)
		if isSmallerScore1(score1, bestScore) {
			bestScore = score1
			bestRoute = route
		}
		fmt.Fprintln(out, "curr Score:", score1)
		fmt.Fprintln(out, "best Score:", bestScore)

	}
	fmt.Fprintln(out, "best Route:", bestRoute)
	return bestRoute, bestScore, nil
}

//calculates score of a route
func getRouteScore(route []string) (sc score) {
	for i := 0; i < len(route)-1; i++ {
		var lp locationPair
		lp.fromID = route[i]
		lp.toID = route[i+1]
		foo := scoreMap[lp]
		sc.cost += foo.cost
		sc.duration += foo.duration
		sc.distance += foo.distance
	}
	return
}

//function to permute all route combinations
//Credits: Permutations.java by Robert Sedgewick and Kevin Wayne
func permute(journeyLocationIDs []string, size int, startLocationID string) {
	if size == 1 {
		var s string
		s += startLocationID
		for _, x := range journeyLocationIDs {
			s += "|" + x
		}
		s += "|" + startLocationID
		routeCombsString += s + "@"
		return
	}
	for i := 0; i < size; i++ {
		swap(journeyLocationIDs, i, size-1)
		permute(journeyLocationIDs, size-1, startLocationID)
		swap(journeyLocationIDs, i, size-1)
	}
}

func swap(routes []string, i int, j int) {
	r := routes[i]
	routes[i] = routes[j]
	routes[j] = r
}

//scoreMap gives the score of every origin to every destination
func generateScoreMap(startLocationID string, journeyLocationIDs []string) error {
	size := len(journeyLocationIDs) + 1
	fmt.Fprintln(out, "size", size)

	fmt.Println("\n1. Fetching location data from MongoLab...")
	startPoint, err := getCords(startLocationID)
	if err != nil {
		return err
	}

	journeyPoints := make([]Point, size-1)
	for i, x := range journeyLocationIDs {
		journeyPoints[i], err = getCords(x)
		if err != nil {
			return err
		}
	}

	fmt.Println("Done.")
	fmt.Println("\n2. Fetching price estimates from UBER...")
	pointArray := make([]Point, size)
	idArray := make([]string, size)
	pointArray[0] = startPoint
	idArray[0] = startLocationID
	for i := 1; i < size; i++ {
		pointArray[i] = journeyPoints[i-1]
		idArray[i] = journeyLocationIDs[i-1]
	}
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			if i != j {
				var lp locationPair
				lp.fromID = idArray[i]
				lp.toID = idArray[j]
				sc := getUberPriceEstimate(pointArray[i], pointArray[j])
				scoreMap[lp] = sc
				fmt.Fprintln(out, "Score of location pair ", lp, " :", scoreMap[lp])
			}

			fmt.Println(arr2to1(size, i, j)+1, "/", (size * size), " done")
		}
	}
	fmt.Println("Done.")
	return nil
}

func arr2to1(s, x, y int) int {
	return (x * s) + y
}

//write http response
func httpResponse(w http.ResponseWriter, jsonOut []byte, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, "%s", jsonOut)
}

//**************IMPLEMENT ERROR HANDLING HERE****************

//get price from UBER api
func getUberPriceEstimate(start, end Point) (sc score) {
	data := buildUberJSON(buildURL(start, end))
	for _, x := range data.Prices {
		if x.DisplayName == "uberX" {
			sc.cost = x.LowEstimate
			sc.duration = x.Duration
			sc.distance = x.Distance
		}
	}
	return
}

//returns the smaller score
func isSmallerScore1(score1, score2 score) bool {
	if score1.cost < score2.cost {
		return true
	} else if score1.cost == score2.cost {
		if score1.duration < score2.duration {
			return true
		} else if score1.duration == score2.duration {
			if score1.distance < score2.distance {
				return true
			}
			return false
		}
		return false
	}
	return false
}

//returns json data from Uber request api
func buildUberEtaJSON(url string, start Point, end Point) (data jsonUberRequest) {
	var reqStruct UberRideRequest
	reqStruct.ProductID = "04a497f5-380d-47f2-bf1b-ad4cfdcb51f2"
	reqStruct.StartLatitude = start.Lat
	reqStruct.StartLongitude = start.Lng
	reqStruct.EndLatitude = end.Lat
	reqStruct.EndLongitude = end.Lng

	bearerToken := "Bearer" + " " + "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzY29wZXMiOlsicmVxdWVzdCJdLCJzdWIiOiI0YTI2MGNjZC1iM2Q1LTQ1NjItYjAzYS01NzIxNTFkODIxYjUiLCJpc3MiOiJ1YmVyLXVzMSIsImp0aSI6IjJjMzU3NWE3LTM4M2UtNDQzYy04YWQ0LTFlYzM0OWFkNGEzOSIsImV4cCI6MTQ1MDIxOTcxMiwiaWF0IjoxNDQ3NjI3NzExLCJ1YWN0IjoiQnR0SG96ZHl2TFRnb2VweVBCa1FYMk9uZG96N2JNIiwibmJmIjoxNDQ3NjI3NjIxLCJhdWQiOiJ1WnlXRHdpLUVBR1lYRU9NdnRoQVQxc29kcFJrLUdLYSJ9.n-pbjqEbjHybI8J6mh8w3GtQkHxqpFUZJlIgAs44xIhE5XVq4vSLEOpEkePZ-3-Q8qXz8B024KYvH0tVbqyFwSTwe-VwnvHJrWe7q2F3mMZ6pOw0pk7hg1hZLdG9fKE3sA5ddKugnj5SVxZ5mFD3b46qSm4e4mJivY-XGK05SD8qSxvlb6HFjMLIvKeOrGkHgctpwmWd5PeIKpgWlFrISXf8Up7RAvfvKKlfdrXnrJXCthoFzgbkKvqdtJP6bP6GfiVB0VTaUHRc3ZrMs1dcRLU8GUnq1jubdZwgHrYG4DNbQ8KtdHJwOFUeB4FjfMPObjxiEIbZGga4qHoXvPzFKQ"

	jsonOut, _ := json.Marshal(reqStruct)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonOut))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerToken)
	client := &http.Client{}

	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		fmt.Println("Panic@buildUberEtaJSON.client.Do")
		panic(err)
	}

	jsonDataFromHTTP, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Panic@buildUberEtaJSON.ioutil.ReadAll")
		panic(err)
	}

	if err := json.Unmarshal([]byte(jsonDataFromHTTP), &data); err != nil {
		fmt.Println("Panic@buildUberEtaJSON.json.Unmarshal")
		panic(err)
	}

	fmt.Fprintln(out, "buildUberEtaJSON().data=", data)
	return
}

//returns json data from UBER price estimate api
func buildUberJSON(url string) (data jsonUberPrice) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Token MV-wnpHB8RSuADk8pq0dkeDjK1DWD_Diu7vEQw_z")

	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		fmt.Println("Panic@buildUberJSON.client.Do")
		panic(err)
	}

	jsonDataFromHTTP, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Panic@buildUberJSON.ioutil.ReadAll")
		panic(err)
	}

	if err := json.Unmarshal([]byte(jsonDataFromHTTP), &data); err != nil {
		fmt.Println("Panic@buildUberJSON.json.Unmarshal")
		panic(err)
	}

	fmt.Fprintln(out, "buildUberJSON().data=", data)
	return
}

//builds URL for UBER call
func buildURL(start, end Point) (url string) {
	url = "https://sandbox-api.uber.com/v1/estimates/price?"
	url += "start_latitude="
	url += strconv.FormatFloat(start.Lat, 'f', -1, 64)
	url += "&start_longitude="
	url += strconv.FormatFloat(start.Lng, 'f', -1, 64)
	url += "&end_latitude="
	url += strconv.FormatFloat(end.Lat, 'f', -1, 64)
	url += "&end_longitude="
	url += strconv.FormatFloat(end.Lng, 'f', -1, 64)
	fmt.Fprintln(out, "UberPriceEstimateURL:", url)
	return
}

//Get trips data for Trip ID
func getTripData(id string) (resp PostGetResponse, err error) {
	if !bson.IsObjectIdHex(id) {
		return resp, errors.New("Invalid ID")
	}

	session, err := getTripSession()
	if err != nil {
		return resp, err
	}

	if err := session.DB("tripsdata").C("trips").FindId(bson.ObjectIdHex(id)).One(&resp); err != nil {
		return resp, errors.New("ID does not exist")
	}
	return resp, nil
}

//returns co-ordinates of a location ID from MongoDb
func getCords(locationID string) (Point, error) {
	var ld LocationData
	if !bson.IsObjectIdHex(locationID) {
		return ld.Coordinate, errors.New("Invalid ID")
	}

	session, err := getLocationSession()
	if err != nil {
		return ld.Coordinate, err
	}
	if err := session.DB("locationdata").C("locations").FindId(bson.ObjectIdHex(locationID)).One(&ld); err != nil {
		return ld.Coordinate, errors.New("ID does not exist")
	}
	return ld.Coordinate, nil
}

func main() {
	//debugging variables----------------------
	debugModeActivated = false //change to true to see all developer messages
	out = ioutil.Discard
	if debugModeActivated {
		out = os.Stdout
	}
	//---------------------debugging variables

	mux := httprouter.New()
	fmt.Println("\nServer ready... Waiting for requests...")
	mux.GET("/trips/:id", CheckTrip)
	mux.POST("/trips", PlanTrip)
	mux.PUT("/trips/:id/request", RequestTrip)
	http.ListenAndServe("localhost:8080", mux)
}
