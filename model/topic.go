package model

import (
	"log"
	"xorm.io/builder"
)

// Author: Junlang
// struct : Topic(大题)
// comment: must capitalize the first letter of the field in Topic
type Topic struct {
	QuestionId    int64  `json:"question_id" xorm:"pk autoincr"`
	QuestionName  string `json:"question_name" xorm:"varchar(50)"`
	SubjectName   string `json:"subject_name" xorm:"varchar(50)"`
	StandardError int64  `json:"standard_error"`
	QuestionScore int64  `json:"question_score"`
	ScoreType     int64  `json:"score_type"`
	ImportNumber  int64  `json:"import_number"`
	ImportTime    string `json:"import_time"`
	SubjectId     int64  `json:"subject_id"`
	SelfScoreRate int64  `json:"self_score_rate"`
}

func (t *Topic) GetTopic(id int64) error {
	has, err := adapter.engine.Where(builder.Eq{"question_id": id}).Get(t)
	if !has || err != nil {
		log.Println("could not find topic")
	}
	return err
}

func GetTopicList(topics *[]Topic) error {
	err := adapter.engine.Find(topics)
	if err != nil {
		log.Println("GetTopicList err ")
	}
	return err
}

func FindTopicBySubNameList(topics *[]Topic, subjectName string) error {
	err := adapter.engine.Where("subject_name=?", subjectName).Find(topics)
	if err != nil {
		log.Println("FindTopicBySubNameList err ")
	}
	return err
}

func FindTopicList(topics *[]Topic) error {
	err := adapter.engine.Find(topics)
	if err != nil {
		log.Println("FindTopicList err ")
	}
	return err
}
func InsertTopic(topic *Topic) (err1 error, questionId int64) {
	_, err := adapter.engine.Insert(topic)
	if err != nil {
		log.Println("GetTopicList err ")
	}

	return err, topic.QuestionId
}

// func Update ( topic *Topic,questionId int64)error {
//	_,err:= adapter.engine.Where("question_id=?",questionId).Update(&topic)
//	if err!=nil {
//		log.Println("Update topic err ")
//	}
//
//	return  err
// }
func (t *Topic) Update() error {
	code, err := adapter.engine.Where(builder.Eq{"question_id": t.QuestionId}).Update(t)
	if code == 0 || err != nil {
		log.Println("update Topic fail")
		log.Printf("%+v", err)
	}
	return err
}

func (t *Topic) Delete() error {
	_, err := adapter.engine.Where("question_id = ?", t.QuestionId).Delete(t)
	return err
}
