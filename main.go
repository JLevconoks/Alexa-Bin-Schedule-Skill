package main

import (
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"log"
	"os"
	"sort"
	"time"
)

const (
	timestampLayout     = "02/01/06"
	speakErr            = "<speak>There was an error, %v</speak>"
	firstSchedulePhrase = "Your next bin is '%v' on '%v', <say-as interpret-as=\"date\">????%v</say-as>. "
	nextSchedulePhrase  = "Then '%v' on '%v', <say-as interpret-as=\"date\">????%v</say-as>. "
)

var errNoSchedule = errors.New("no schedule available")

type Response struct {
	Version string       `json:"version"`
	Body    ResponseBody `json:"response"`
}

type ResponseBody struct {
	OutputSpeech     Payload `json:"outputSpeech,omitempty"`
	ShouldEndSession bool    `json:"shouldEndSession"`
}

type Payload struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
	Ssml string `json:"ssml"`
}

type BinScheduleDao struct {
	DocumentId string   `json:"documentId"`
	PremisesId string   `json:"premisesId"`
	Brown      []string `json:"brown"`
	Green      []string `json:"green"`
	Black      []string `json:"black"`
}

type scheduleItem struct {
	colour string
	date   time.Time
}

func main() {
	//response, _ := MyBinCollectionHandler()
	//log.Println(response)
	lambda.Start(MyBinCollectionHandler)
}

func MyBinCollectionHandler() (Response, error) {
	premisesId, exist := os.LookupEnv("premisesid")
	if !exist {
		log.Fatal("Missing 'premisesid' parameter")
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("eu-west-1"),
	})

	if err != nil {
		log.Fatal(err)
	}

	db := dynamodb.New(sess)
	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String("BinSchedule"),
		Key: map[string]*dynamodb.AttributeValue{
			"documentId": {S: aws.String(fmt.Sprintf("LEEDS_%v", premisesId))},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	rawSchedules := BinScheduleDao{}

	err = dynamodbattribute.UnmarshalMap(result.Item, &rawSchedules)
	if err != nil {
		log.Println("Error while unmarshalling", err)
		return responseWithError(err.Error()), nil
	}
	schedules, err := rawSchedules.getNextSchedules()
	if err != nil {
		log.Println("Error processing schedules", err)
		return responseWithError(err.Error()), nil
	}

	rsp := buildScheduleResponse(schedules)
	return rsp, nil

}

func (dao BinScheduleDao) getNextSchedules() ([]scheduleItem, error) {
	schedules := make([]scheduleItem, 0)
	blackTime, err := nextScheduled(dao.Black)
	if err != nil && err != errNoSchedule {
		return nil, err
	}
	schedules = append(schedules, scheduleItem{
		colour: "black",
		date:   blackTime,
	})

	greenTime, err := nextScheduled(dao.Green)
	if err != nil && err != errNoSchedule {
		return nil, err
	}
	schedules = append(schedules, scheduleItem{
		colour: "green",
		date:   greenTime,
	})

	brownTime, err := nextScheduled(dao.Brown)
	if err != nil && err != errNoSchedule {
		return nil, err
	}
	schedules = append(schedules, scheduleItem{
		colour: "brown",
		date:   brownTime,
	})

	sort.Slice(schedules, func(i, j int) bool {
		return schedules[i].date.Before(schedules[j].date)
	})

	return schedules, nil
}

func nextScheduled(rawSchedule []string) (time.Time, error) {
	schedules := make([]time.Time, 0)

	ignoreBeforeTime := time.Now().Add(-24 * time.Hour)
	for _, s := range rawSchedule {
		scheduleTime, err := time.Parse(timestampLayout, s)
		if err != nil {
			return time.Time{}, errors.New(fmt.Sprintf("error parsing Date %v", s))
		}

		if scheduleTime.After(ignoreBeforeTime) {
			schedules = append(schedules, scheduleTime)
		}
	}

	if len(schedules) > 1 {
		sort.Slice(schedules, func(i, j int) bool {
			return schedules[i].Before(schedules[j])
		})
	}

	if len(schedules) == 0 {
		return time.Time{}, errNoSchedule
	}

	return schedules[0], nil
}

func buildScheduleResponse(schedules []scheduleItem) Response {
	response := ""

	if len(schedules) == 0 {
		return responseWithError("No schedules found in the database")
	}

	for i, s := range schedules {
		if i == 0 {
			response += fmt.Sprintf(firstSchedulePhrase, s.colour, s.date.Format("Monday"), s.date.Format("0102"))
			continue
		}

		response += fmt.Sprintf(nextSchedulePhrase, s.colour, s.date.Format("Monday"), s.date.Format("0102"))
	}
	return buildResponse(fmt.Sprintf("<speak><amazon:domain name=\"long-form\">%v</amazon:domain></speak>", response))
}

func buildResponse(rsp string) Response {
	return Response{
		Version: "1.0",
		Body: ResponseBody{
			OutputSpeech: Payload{
				Type: "SSML",
				Ssml: rsp,
			},
			ShouldEndSession: true,
		},
	}
}

func responseWithError(errmsg string) Response {
	speakResponse := fmt.Sprintf(speakErr, errmsg)
	return buildResponse(speakResponse)
}
