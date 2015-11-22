# Trip Planner
This program takes a set of locations from the Location Service database and then checks against UBER’s price estimates API to suggest the best possible route in terms of costs and duration.

Uses UBER Sandbox environment for all API calls.
 
Implements these endpoints in order to take orchestrate between user and UBER services.

### Plan a trip
```http
POST /trips 
```
Request:
```json 
{
 "starting_from_location_id" : "999999",
 "location_ids" : [ "10000", "10001", "20004", "30003" ] 
}
```
Response: 
```http
HTTP 201
```
```json
{
 "id" : "1122",
 "status" : "planning",
 "starting_from_location_id": "999999",
 "best_route_location_ids" : [ "30003", "10001", "10000", "20004" ],
 "total_uber_costs" : 125,
 "total_uber_duration" : 640,
 "total_distance" : 25.05 
}
```
### Check the trip details and status
```http
GET /trips/{trip_id} 
```
Request:
```http
GET /trips/1122
```
Response:
```json
{
 "id" : "1122",
 "status" : "planning",
 "starting_from_location_id": "999999",
 "best_route_location_ids" : [ "30003", "10001", "10000", "20004" ],
 "total_uber_costs" : 125,
 "total_uber_duration" : 640,
 "total_distance" : 25.05 
}
```
### Start the trip by requesting UBER for the first destination. Calls UBER request API (sandbox) to request a car from starting point to the next destination (simulation).
```http
PUT /trips/{trip_id}/request 
```
UBER Request API:
```http
PUT /v1/sandbox/requests/{request_id}
```
Once a destination is reached, the subsequent calls to API will request a car for the next destination.

Request:
```http
PUT /trips/1122/request
```
Response:
```json
{
 "id" : "1122",
 "status" : "requesting",
 "starting_from_location_id": "999999",
 "next_destination_location_id" : "30003",
 "best_route_location_ids" : [ "30003", "10001", "10000", "20004" ],
 "total_uber_costs" : 125,
 "total_uber_duration" : 640,
 "total_distance" : 25.05,
 "uber_wait_time_eta" : 5 
}
```
Last Response:
```json
{
 "id" : "1122",
 "status" : "finished",
 "starting_from_location_id": "999999",
 "next_destination_location_id": "",
 "best_route_location_ids" : [ "30003", "10001", "10000", "20004" ],
 "total_uber_costs" : 125,
 "total_uber_duration" : 640,
 "total_distance" : 25.05,
 "uber_wait_time_eta" : 5 
}
```
