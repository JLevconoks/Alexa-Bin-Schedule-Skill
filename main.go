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

type binSchedule struct {
	DocumentId string
	PremisesId string
	schedule   []scheduleItem
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

	r := BinScheduleDao{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &r)
	if err != nil {
		log.Println("Error while unmarshalling", err)
	}

	schedule, err := r.toBinSchedule()
	response := ""
	if err != nil {
		response = fmt.Sprintf("<speak>There was an error, '%v'</speak>", err)
	} else {
		date := schedule.schedule[0].date
		response = fmt.Sprintf("<speak>Your next bin is '%v', on %v <say-as interpret-as=\"date\">????%v</say-as></speak>", schedule.schedule[0].colour, date.Format("Monday"), date.Format("0102"))
	}
	return buildResponse(response), nil

}

func (dao BinScheduleDao) toBinSchedule() (binSchedule, error) {
	result := binSchedule{DocumentId: dao.DocumentId, PremisesId: dao.PremisesId}

	layout := "02/01/06"

	for _, v := range dao.Black {
		t, err := time.Parse(layout, v)
		if err != nil {
			return binSchedule{}, errors.New(fmt.Sprintf("Error parsing Date %v", v))

		}
		result.schedule = append(result.schedule, scheduleItem{"Black", t})
	}

	for _, v := range dao.Green {
		t, err := time.Parse(layout, v)
		if err != nil {
			return binSchedule{}, errors.New(fmt.Sprintf("Error parsing Date %v", v))
		}
		result.schedule = append(result.schedule, scheduleItem{"Green", t})
	}

	for _, v := range dao.Brown {
		t, err := time.Parse(layout, v)
		if err != nil {
			return binSchedule{}, errors.New(fmt.Sprintf("Error parsing Date %v", v))
		}
		result.schedule = append(result.schedule, scheduleItem{"Brown", t})
	}

	sort.Slice(result.schedule, func(i, j int) bool {
		return result.schedule[i].date.Before(result.schedule[j].date)
	})

	return result, nil
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
