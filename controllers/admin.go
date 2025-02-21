package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/open-ct/openscore/model"
	"github.com/open-ct/openscore/util"
	"github.com/xuri/excelize/v2"
)

func (c *ApiController) ListTestPaperInfo() {
	var req ListTestPaperInfoRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	var testPaperInfos []model.TestPaperInfo
	err := model.GetTestInfoListByTestId(req.TestId, &testPaperInfos)
	if err != nil {
		resp := Response{"10006", "get testPaperInfo fail", err}
		c.Data["json"] = resp
		return
	}

	c.ResponseOk(testPaperInfos)
}

func (c *ApiController) ListSchools() {
	schools, err := model.ListSchools()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(schools)
}

func (c *ApiController) UpdateUserQualified() {
	var req UpdateUserQualifiedRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	user, err := model.GetUserByAccount(req.Account)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	user.IsQualified = true
	if err := user.UpdateCols("is_qualified"); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk()
}

func (c *ApiController) ListGroupGrades() {
	var req ListGroupGradesRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	group, err := model.GetGroupByGroupId(req.GroupId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	resp := ListGroupGradesResponse{
		GroupName: group.GroupName,
	}

	for _, id := range group.TestIds {
		score, err := model.GetTestPaperTeachingScoreById(id)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		resp.Scores = append(resp.Scores, score)
	}

	userPaperList, err := model.ListUserPaperGroupByGroupId(group.Id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	for _, userPaper := range userPaperList {

		scores, err := model.ListTeacherScoreByTestIds(userPaper.UserId, group.TestIds)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		var num float32 = 0
		for i, score := range resp.Scores {
			if score == scores[i] {
				num++
			}
		}

		var u model.User
		if err := u.GetUser(userPaper.UserId); err != nil {
			c.ResponseError(err.Error())
			return
		}

		resp.TeacherGrades = append(resp.TeacherGrades, TeacherGrade{
			TeacherAccount:  u.Account,
			ConcordanceRate: num / float32(len(group.TestIds)),
			Scores:          scores,
			IsQualified:     u.IsQualified,
		})
	}

	c.ResponseOk(resp)
}

func (c *ApiController) DeletePaperFromGroup() {
	var req DeletePaperFromGroupRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	group, err := model.GetGroupByGroupId(req.GroupId)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	for i, id := range group.TestIds {
		if id == req.TestId {
			group.TestIds = append(group.TestIds[:i], group.TestIds[i+1:]...)
			break
		}
	}

	if err := group.Update(); err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk()
}

func (c *ApiController) ListTestPapersByQuestionId() {
	var req ListTestPapersRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	testPapers := make([]model.TestPaper, 0)
	if err := model.FindTestPaperByQuestionId(req.QuestionId, &testPapers); err != nil {
		c.ResponseError("FindTestPaperByQuestionId", err)
		return
	}

	respTestPapers := testPapers

	if req.School != "" {
		respTestPapers = nil
		for _, paper := range testPapers {
			if paper.School == req.School {
				respTestPapers = append(respTestPapers, paper)
			}
		}
	}

	if req.TicketId != "" {
		respTestPapers = nil
		for _, paper := range testPapers {
			if paper.TicketId == req.TicketId {
				respTestPapers = append(respTestPapers, paper)
			}
		}
	}

	c.ResponseOk(respTestPapers)
}

func (c *ApiController) ListPaperGroups() {
	groups, err := model.ListPaperGroup()
	if err != nil {
		c.ResponseError("cannot ListPaperGroup", err)
		return
	}

	resp := ListPaperGroupsResponse{}
	for _, group := range groups {
		groupInfo := PaperGroupInfo{
			GroupId:   group.Id,
			GroupName: group.GroupName,
			Papers:    nil,
		}
		for _, id := range group.TestIds {
			testPaper := model.TestPaper{}
			if err := testPaper.GetTestPaperByTestId(id); err != nil {
				c.ResponseError(err.Error())
				return
			}

			groupInfo.Papers = append(groupInfo.Papers, testPaper)
		}

		resp.Groups = append(resp.Groups, groupInfo)
	}

	c.ResponseOk(resp)
}

func (c *ApiController) TeachingPaperGrouping() {
	var req TeachingGroupRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	testIds := make([]int64, len(req.Papers))

	for i, paper := range req.Papers {
		testIds[i] = paper.TestId
		testPaper := model.TestPaper{}
		if err := testPaper.GetTestPaperByTestId(paper.TestId); err != nil {
			c.ResponseError(err.Error())
			return
		}

		testPaper.TeachingScore = 0
		for _, score := range paper.Scores {
			testPaper.TeachingScore += score
		}

		if err := testPaper.Update(); err != nil {
			c.ResponseError("Update testPaper", err)
			return
		}

	}

	group := model.PaperGroup{
		GroupName: req.GroupName,
		TestIds:   testIds,
	}

	if err := model.CreatePaperGroup(&group); err != nil {
		c.ResponseError("CreatePaperGroup", err)
		return
	}

	c.ResponseOk()
}

func (c *ApiController) CreateSmallQuestion() {
	var req CreateSmallQuestionRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	subTopic := model.SubTopic{
		QuestionDetailName:  req.QuestionDetailName,
		QuestionId:          req.QuestionId,
		QuestionDetailScore: req.QuestionDetailScore,
		ScoreType:           req.ScoreType,
		IsSecondScore:       req.IsSecondScore,
	}

	if err := model.InsertSubTopic(&subTopic); err != nil {
		c.ResponseError("insert subTopic fail", err)
		return
	}

	c.ResponseOk(subTopic)
}

func (c *ApiController) DeleteSmallQuestion() {
	var req DeleteSmallQuestionRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	var subTopic model.SubTopic
	if err := subTopic.GetSubTopic(req.QuestionDetailId); err != nil {
		c.ResponseError("get subTopic fail", err)
		return
	}

	if err := subTopic.Delete(); err != nil {
		c.ResponseError("delete subTopic fail", err)
		return
	}

	var topic model.Topic
	topic.GetTopic(subTopic.QuestionId)
	topic.QuestionScore -= subTopic.QuestionDetailScore
	topic.Update()
	c.ResponseOk()
}

func (c *ApiController) UpdateSmallQuestion() {
	var req UpdateSmallQuestionRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	var subTopic model.SubTopic
	if err := subTopic.GetSubTopic(req.QuestionDetailId); err != nil {
		c.ResponseError("get subTopic fail", err)
		return
	}

	subTopic.QuestionDetailName = req.QuestionDetailName
	subTopic.ScoreType = req.ScoreType
	subTopic.IsSecondScore = req.IsSecondScore
	subTopic.QuestionDetailScore = req.QuestionDetailScore

	if err := subTopic.Update(); err != nil {
		c.ResponseError("update subTopic fail", err)
		return
	}

	var topic model.Topic
	topic.GetTopic(subTopic.QuestionId)
	topic.QuestionScore -= subTopic.QuestionDetailScore - req.QuestionDetailScore
	topic.Update()

	c.ResponseOk()
}

func (c *ApiController) DeleteQuestion() {
	var req DeleteQuestionRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	var topic model.Topic
	if err := topic.GetTopic(req.QuestionId); err != nil {
		c.ResponseError("get topic fail", err)
		return
	}

	if err := topic.Delete(); err != nil {
		c.ResponseError("delete topic fail", err)
		return
	}

	c.ResponseOk()
}

func (c *ApiController) UpdateQuestion() {
	var req UpdateQuestionRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	var topic model.Topic
	if err := topic.GetTopic(req.QuestionId); err != nil {
		c.ResponseError("get topic fail", err)
		return
	}

	topic.QuestionName = req.QuestionName
	topic.ScoreType = req.ScoreType
	topic.StandardError = req.StandardError
	topic.QuestionScore = req.QuestionScore

	if err := topic.Update(); err != nil {
		c.ResponseError("update topic fail", err)
		return
	}

	c.ResponseOk()
}

func (c *ApiController) CreateUser() {
	var req CreateUserRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	account := req.SubjectName + "10000"
	u := model.GetLastUserBySubject(req.SubjectName)
	if u != nil {
		res := strings.Split(u.Account, req.SubjectName)
		id, err := strconv.Atoi(res[1])
		if err != nil {
			c.ResponseError("cannot resolve account", err)
			return
		}
		account = req.SubjectName + strconv.Itoa(id+1)
	}

	user := &model.User{
		Account:     account,
		UserName:    req.UserName,
		Password:    req.Password,
		SubjectName: req.SubjectName,
		QuestionId:  req.QuestionId,
		UserType:    req.UserType,
	}

	if err := user.Insert(); err != nil {
		c.ResponseError("insert user", err)
		return
	}

	c.ResponseOk()
}

func (c *ApiController) DeleteUser() {
	var req DeleteUserRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	u, err := model.GetUserByAccount(req.Account)
	if err != nil {
		c.ResponseError("cant get user", err)
		return
	}

	if err := u.Delete(); err != nil {
		c.ResponseError("delete user", err)
		return
	}

	c.ResponseOk()
}

func (c *ApiController) UpdateUser() {
	var req UpdateUserRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	u, err := model.GetUserByAccount(req.Account)
	if err != nil {
		c.ResponseError("cant get user", err)
		return
	}

	u.UserType = req.UserType
	u.UserName = req.UserName
	u.SubjectName = req.SubjectName
	u.Password = req.Password
	u.IsAttempt = req.IsAttempt

	if err := u.UpdateCols("user_type", "user_name", "subject_name", "password", "is_attempt"); err != nil {
		c.ResponseError("update user error", err)
		return
	}

	c.ResponseOk()
}

func (c *ApiController) ListUsers() {
	users, err := model.ListUsers()
	if err != nil {
		c.ResponseError("list users error", err)
		return
	}

	c.ResponseOk(users)
}

// WriteUserExcel 导出用户
func (c *ApiController) WriteUserExcel() {
	var req WriteUserRequest

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.ResponseError("cannot unmarshal", err)
		return
	}

	// subjectName, err := model.GetSubjectById(req.SubjectId)
	// if err != nil {
	// 	c.ResponseError(err.Error())
	// 	return
	// }
	// if subjectName == "" {
	// 	c.ResponseError("cant get subjectName")
	// 	return
	// }
	subjectName := req.SubjectName

	f := excelize.NewFile()
	// Create a new sheet.
	activeSheet := f.NewSheet(subjectName)

	// Set value of a cell.
	f.SetCellValue(subjectName, "A1", "账号")
	f.SetCellValue(subjectName, "B1", "密码")
	f.SetCellValue(subjectName, "C1", "学科")
	f.SetCellValue(subjectName, "D1", "身份")
	f.SetCellValue(subjectName, "E1", "大题号")
	// Set active sheet of the workbook.
	f.SetActiveSheet(activeSheet)

	for i := 0; i < req.SupervisorNumber; i++ {
		index := 2 + i
		f.SetCellValue(subjectName, "A"+strconv.Itoa(index), "n"+req.SubjectName+strconv.Itoa(10000+i))
		f.SetCellValue(subjectName, "B"+strconv.Itoa(index), "123")
		f.SetCellValue(subjectName, "C"+strconv.Itoa(index), subjectName)
		f.SetCellValue(subjectName, "D"+strconv.Itoa(index), "阅卷组长")

		u := model.User{
			Account:        "n" + subjectName + strconv.Itoa(10000+i),
			Password:       "123",
			UserName:       "n" + subjectName + strconv.Itoa(10000+i),
			SubjectName:    subjectName,
			IsOnlineStatus: false,
			QuestionId:     0,
			UserType:       "supervisor",
		}
		if err := u.Insert(); err != nil {
			c.ResponseError(err.Error())
		}
	}

	index := req.SupervisorNumber + 2
	for _, item := range req.List {
		for i := 0; i < item.Num; i++ {
			f.SetCellValue(subjectName, "A"+strconv.Itoa(index), "s"+req.SubjectName+strconv.Itoa(10000+index))
			f.SetCellValue(subjectName, "B"+strconv.Itoa(index), "123")
			f.SetCellValue(subjectName, "C"+strconv.Itoa(index), subjectName)
			f.SetCellValue(subjectName, "D"+strconv.Itoa(index), "阅卷员")
			f.SetCellValue(subjectName, "E"+strconv.Itoa(index), item.Id)

			u := model.User{
				Account:        "s" + subjectName + strconv.Itoa(10000+index),
				Password:       "123",
				UserName:       "s" + subjectName + strconv.Itoa(10000+index),
				SubjectName:    subjectName,
				IsOnlineStatus: false,
				QuestionId:     item.Id,
				UserType:       "normal",
			}
			if err := u.Insert(); err != nil {
				c.ResponseError(err.Error())
			}

			index++
		}
	}

	// Save spreadsheet by the given path.
	if err := f.SaveAs("./tmp/users.xlsx"); err != nil {
		c.ResponseError(err.Error(), "users 表导出错误")
		return
	}

	c.Ctx.Output.Download("./tmp/users.xlsx", "users.xlsx")
}

// ReadUserExcel 导入用户
// func (c *ApiController) ReadUserExcel() {
// 	c.Ctx.ResponseWriter.Header().Set("Access-Control-Allow-Origin", c.Ctx.Request.Header.Get("Origin"))
// 	defer c.ServeJSON()
//
// 	file, header, err := c.GetFile("excel")
//
// 	if err != nil {
// 		log.Println(err)
// 		c.Data["json"] = Response{Status: "10001", Msg: "cannot unmarshal", Data: err}
// 		return
// 	}
// 	tempFile, err := os.Create(header.Filename)
// 	io.Copy(tempFile, file)
// 	f, err := excelize.OpenFile(header.Filename)
// 	if err != nil {
// 		log.Println(err)
// 		c.Data["json"] = Response{Status: "30000", Msg: "excel 表导入错误", Data: err}
// 		return
// 	}
//
// 	// Get all the rows in the Sheet1.
// 	rows, err := f.GetRows("Sheet1")
// 	if err != nil {
// 		log.Println(err)
// 		c.Data["json"] = Response{Status: "30000", Msg: "excel 表导入错误", Data: err}
// 		return
// 	}
//
// 	for _, r := range rows[1:] {
// 		row := make([]string, len(rows[0]))
// 		copy(row, r)
// 		var user model.User
// 		user.UserName = row[0]
// 		user.IdCard = row[1]
// 		user.Password = row[2]
// 		user.Tel = row[3]
// 		user.Address = row[4]
// 		user.SubjectName = row[5]
// 		user.Email = row[6]
// 		userType, _ := strconv.Atoi(row[7])
// 		user.UserType = int64(userType)
// 		if err := user.Insert(); err != nil {
// 			log.Println(err)
// 			c.Data["json"] = Response{Status: "30001", Msg: "用户导入错误", Data: err}
// 			return
// 		}
//
// 	}
//
// 	err = tempFile.Close()
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	err = os.Remove(header.Filename)
// 	if err != nil {
// 		log.Println(err)
// 	}
//
// 	// ------------------------------------------------
// 	data := make(map[string]interface{})
// 	data["data"] = nil
// 	c.Data["json"] = Response{Status: "10000", Msg: "OK", Data: data}
// }

/**
2.试卷导入
*/
func (c *ApiController) ReadExcel() {
	c.Ctx.ResponseWriter.Header().Set("Access-Control-Allow-Origin", c.Ctx.Request.Header.Get("Origin"))
	defer c.ServeJSON()
	var resp Response

	file, header, err := c.GetFile("excel")

	if err != nil {
		log.Println(err)
		resp = Response{"10001", "cannot unmarshal", err}
		c.Data["json"] = resp
		return
	}
	tempFile, err := os.Create(header.Filename)
	io.Copy(tempFile, file)
	f, err := excelize.OpenFile(header.Filename)
	if err != nil {
		log.Println(err)
		resp = Response{"30000", "excel 表导入错误", err}
		c.Data["json"] = resp
		return
	}

	// Get all the rows in the Sheet1.
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		log.Println(err)
		resp = Response{"30000", "excel 表导入错误", err}
		c.Data["json"] = resp
		return
	}

	var bigQuestions []question
	var smallQuestions []question

	// 处理第一行 获取大题和小题的分布情况
	for i := 8; i < len(rows[0]); i++ {
		questionIds := strings.Split(rows[0][i], "-")

		id0, _ := strconv.Atoi(questionIds[0])
		id1, _ := strconv.Atoi(questionIds[1])
		if len(bigQuestions) > 0 && bigQuestions[len(bigQuestions)-1].Id == id0 {
			bigQuestions[len(bigQuestions)-1].Num++
		} else {
			bigQuestions = append(bigQuestions, question{Id: id0, Num: 1})
		}

		if len(smallQuestions) > 0 && smallQuestions[len(smallQuestions)-1].FatherId == id0 && smallQuestions[len(smallQuestions)-1].Id == id1 {
			smallQuestions[len(smallQuestions)-1].Num++
		} else {
			smallQuestions = append(smallQuestions, question{Id: id1, FatherId: id0, Num: 1})
		}
	}

	subjectName := rows[1][5]
	var topics []model.Topic
	if err := model.FindTopicBySubNameList(&topics, subjectName); err != nil {
		c.ResponseError("get topic list err", err)
		return
	}

	if len(topics) != len(bigQuestions) {
		fmt.Println("----- len(topics): ", len(topics), " -----")
		fmt.Println("----- len(bigQuestions): ", len(bigQuestions), " -----")
		c.ResponseError("len(topics) != len(bigQuestions)")
		return
	}

	smallIndex := 0
	for i, topic := range topics {
		// TODO 测试 可能有顺序问题
		fmt.Println("bigQuestions[i].Id: ", bigQuestions[i].Id)
		fmt.Println("topic.QuestionId: ", topic.QuestionId)

		topic.ImportNumber = int64(len(rows) - 1)
		bigQuestions[i].Id = int(topic.QuestionId) // id映射

		if err := topic.Update(); err != nil {
			c.ResponseError("大题导入试卷数更新错误", err)
			return
		}

		var subTopics []model.SubTopic
		if err := model.FindSubTopicsByQuestionId(topic.QuestionId, &subTopics); err != nil {
			c.ResponseError("get subTopic list err", err)
			return
		}
		for _, subTopic := range subTopics {
			fmt.Println("smallQuestions[smallIndex].Id: ", smallQuestions[smallIndex].Id)
			fmt.Println("subTopic.QuestionDetailId: ", subTopic.QuestionDetailId)

			smallQuestions[smallIndex].Id = int(subTopic.QuestionDetailId)
			smallIndex++
		}
	}

	for _, r := range rows[1:] {
		row := make([]string, len(rows[0]))
		copy(row, r)
		index := 0
		smallIndex := 0
		// 处理该行的大题
		for _, bigQuestion := range bigQuestions {
			var testPaper model.TestPaper
			testPaper.TicketId = row[0]
			testPaper.QuestionId = int64(bigQuestion.Id)
			testPaper.Mobile = row[1]
			isParent, _ := strconv.Atoi(row[2])
			testPaper.IsParent = int64(isParent)
			testPaper.ClientIp = row[3]
			testPaper.Tag = row[4]
			testPaper.Candidate = row[6]
			testPaper.School = row[7]

			testId, err := testPaper.Insert()
			if err != nil {
				c.ResponseError("试卷大题导入错误", err)
				return
			}
			// 处理该大题的小题
			for num := smallIndex + bigQuestion.Num; smallIndex < num; smallIndex++ {
				content := row[index+8]
				for n := index + smallQuestions[smallIndex].Num - 1; index < n; index++ {

					content += "\n" + row[index+9]
					num--
				}

				timestamp := strconv.Itoa(int(time.Now().Unix()))
				src := util.UploadPic(timestamp+row[0]+rows[0][8+index], content)

				var testPaperInfo model.TestPaperInfo
				testPaperInfo.TicketId = row[0]
				testPaperInfo.PicSrc = src

				testPaperInfo.TestId = testId
				testPaperInfo.QuestionDetailId = int64(smallQuestions[smallIndex].Id)

				if err := testPaperInfo.Insert(); err != nil {
					c.ResponseError("试卷小题导错误", err)
					return
				}
				index++
			}

		}
	}

	err = tempFile.Close()
	if err != nil {
		log.Println(err)
	}
	err = os.Remove(header.Filename)
	if err != nil {
		log.Println(err)
	}

	// ------------------------------------------------
	resp = Response{"10000", "OK", nil}
	c.Data["json"] = resp

}

type question struct {
	Id       int
	Num      int
	FatherId int
}

/**
样卷导入
*/

func (c *ApiController) ReadExampleExcel() {
	c.Ctx.ResponseWriter.Header().Set("Access-Control-Allow-Origin", c.Ctx.Request.Header.Get("Origin"))
	defer c.ServeJSON()
	var resp Response

	// ----------------------------------------------------

	file, header, err := c.GetFile("excel")
	if err != nil {
		log.Println(err)
		resp = Response{"10001", "cannot unmarshal", err}
		c.Data["json"] = resp
		return
	}
	tempFile, err := os.Create(header.Filename)
	io.Copy(tempFile, file)
	f, err := excelize.OpenFile(header.Filename)
	if err != nil {
		log.Println(err)
		resp = Response{"30000", "excel 表导入错误", err}
		c.Data["json"] = resp
		return
	}

	// Get all the rows in the Sheet1.
	rows, err := f.GetRows("Sheet2")
	if err != nil {
		log.Println(err)
		resp = Response{"30000", "excel 表导入错误", err}
		c.Data["json"] = resp
		return
	}

	for i := 1; i < len(rows); i++ {
		for j := 1; j < len(rows[i]); j++ {

			if i >= 1 && j >= 3 {
				// 准备数据
				testIdStr := rows[i][0]
				testId, _ := strconv.ParseInt(testIdStr, 10, 64)
				questionIds := strings.Split(rows[0][j], "-")
				questionIdStr := questionIds[0]
				questionId, _ := strconv.ParseInt(questionIdStr, 10, 64)
				questionDetailIdStr := questionIds[3]
				questionDetailId, _ := strconv.ParseInt(questionDetailIdStr, 10, 64)
				name := rows[i][2]
				// 填充数据
				var testPaperInfo model.TestPaperInfo
				var testPaper model.TestPaper

				testPaperInfo.QuestionDetailId = questionDetailId
				s := rows[i][j]
				// split := strings.Split(s, "\n")

				timestamp := strconv.Itoa(int(time.Now().Unix()))
				src := util.UploadPic(timestamp+rows[i][0]+rows[0][j], s)
				testPaperInfo.PicSrc = src
				// 查看大题试卷是否已经导入
				has, err := testPaper.GetTestPaper(testId)
				if err != nil {
					log.Println(err)
				}

				// 导入大题试卷
				if !has {
					testPaper.TestId = testId
					testPaper.QuestionId = questionId
					testPaper.QuestionStatus = 6
					testPaper.Candidate = name
					_, err = testPaper.Insert()
					if err != nil {
						log.Println(err)
						resp = Response{"30001", "试卷大题导入错误", err}
						c.Data["json"] = resp
						return
					}
				}
				// 导入小题试卷
				testPaperInfo.TestId = testId
				err = testPaperInfo.Insert()
				if err != nil {
					log.Println(err)
					resp = Response{"30002", "试卷小题导错误", err}
					c.Data["json"] = resp
					return
				}

			}

		}

	}
	// 获取选项名 存导入试卷数
	for k := 3; k < len(rows[0]); k++ {
		questionIds := strings.Split(rows[0][k], "-")
		questionIdStr := questionIds[0]
		questionId, _ := strconv.ParseInt(questionIdStr, 10, 64)
		var topic model.Topic
		topic.QuestionId = questionId
		topic.ImportNumber = int64(len(rows) - 1)
		err = topic.Update()
		if err != nil {
			log.Println(err)
			resp = Response{"30003", "大题导入试卷数更新错误", err}
			c.Data["json"] = resp
			return
		}
	}

	err = tempFile.Close()
	if err != nil {
		log.Println(err)
	}
	err = os.Remove(header.Filename)
	if err != nil {
		log.Println(err)
	}

	// ------------------------------------------------
	data := make(map[string]interface{})
	data["data"] = nil
	resp = Response{"10000", "OK", data}
	c.Data["json"] = resp
}

func (c *ApiController) ReadAnswerExcel() {
	c.Ctx.ResponseWriter.Header().Set("Access-Control-Allow-Origin", c.Ctx.Request.Header.Get("Origin"))
	defer c.ServeJSON()
	var resp Response

	// ----------------------------------------------------

	file, header, err := c.GetFile("excel")
	if err != nil {
		log.Println(err)
		resp = Response{"10001", "cannot unmarshal", err}
		c.Data["json"] = resp
		return
	}
	tempFile, err := os.Create(header.Filename)
	io.Copy(tempFile, file)
	f, err := excelize.OpenFile(header.Filename)
	if err != nil {
		log.Println(err)
		resp = Response{"30000", "excel 表导入错误", err}
		c.Data["json"] = resp
		return
	}

	// Get all the rows in the Sheet1.
	rows, err := f.GetRows("Sheet2")
	if err != nil {
		log.Println(err)
		resp = Response{"30000", "excel 表导入错误", err}
		c.Data["json"] = resp
		return
	}

	for i := 1; i < len(rows); i++ {
		for j := 1; j < len(rows[i]); j++ {

			if i >= 1 && j >= 3 {
				// 准备数据
				testIdStr := rows[i][0]
				testId, _ := strconv.ParseInt(testIdStr, 10, 64)
				questionIds := strings.Split(rows[0][j], "-")
				questionIdStr := questionIds[0]
				questionId, _ := strconv.ParseInt(questionIdStr, 10, 64)
				questionDetailIdStr := questionIds[3]
				questionDetailId, _ := strconv.ParseInt(questionDetailIdStr, 10, 64)
				name := rows[i][2]
				// 填充数据
				var testPaperInfo model.TestPaperInfo
				var testPaper model.TestPaper

				testPaperInfo.QuestionDetailId = questionDetailId
				s := rows[i][j]
				// split := strings.Split(s, "\n")

				timestamp := strconv.Itoa(int(time.Now().Unix()))
				src := util.UploadPic(timestamp+rows[i][0]+rows[0][j], s)
				testPaperInfo.PicSrc = src
				// 查看大题试卷是否已经导入
				has, err := testPaper.GetTestPaper(testId)
				if err != nil {
					log.Println(err)
				}

				// 导入大题试卷
				if !has {
					testPaper.TestId = testId
					testPaper.QuestionId = questionId
					testPaper.QuestionStatus = 5
					testPaper.Candidate = name
					_, err = testPaper.Insert()
					if err != nil {
						log.Println(err)
						resp = Response{"30001", "试卷大题导入错误", err}
						c.Data["json"] = resp
						return
					}
				}
				// 导入小题试卷
				testPaperInfo.TestId = testId
				err = testPaperInfo.Insert()
				if err != nil {
					log.Println(err)
					resp = Response{"30002", "试卷小题导错误", err}
					c.Data["json"] = resp
					return
				}

			}

		}

	}
	// 获取选项名 存导入试卷数
	for k := 3; k < len(rows[0]); k++ {
		questionIds := strings.Split(rows[0][k], "-")
		questionIdStr := questionIds[0]
		questionId, _ := strconv.ParseInt(questionIdStr, 10, 64)
		var topic model.Topic
		topic.QuestionId = questionId
		topic.ImportNumber = int64(len(rows) - 1)
		err = topic.Update()
		if err != nil {
			log.Println(err)
			resp = Response{"30003", "大题导入试卷数更新错误", err}
			c.Data["json"] = resp
			return
		}
	}

	err = tempFile.Close()
	if err != nil {
		log.Println(err)
	}
	err = os.Remove(header.Filename)
	if err != nil {
		log.Println(err)
	}

	// ------------------------------------------------
	data := make(map[string]interface{})
	data["data"] = nil
	resp = Response{"10000", "OK", data}
	c.Data["json"] = resp

}

/**
3.大题列表
*/

func (c *ApiController) QuestionBySubList() {
	defer c.ServeJSON()
	var req QuestionBySubList
	var resp Response

	err := json.Unmarshal(c.Ctx.Input.RequestBody, &req)
	if err != nil {
		log.Println(err)
		resp = Response{"10001", "cannot unmarshal", err}
		c.Data["json"] = resp
		return
	}
	// supervisorId := req.SupervisorId
	subjectName := req.SubjectName
	// ----------------------------------------------------
	// 获取大题列表
	topics := make([]model.Topic, 0)
	err = model.FindTopicBySubNameList(&topics, subjectName)
	if err != nil {
		log.Println(err)
		resp = Response{"30004", "获取大题列表错误  ", err}
		c.Data["json"] = resp
		return
	}

	var questions = make([]QuestionBySubListVO, len(topics))
	for i := 0; i < len(topics); i++ {

		questions[i].QuestionId = topics[i].QuestionId
		questions[i].QuestionName = topics[i].QuestionName

	}

	// ----------------------------------------------------
	data := make(map[string]interface{})
	data["questionsList"] = questions
	resp = Response{"10000", "OK", data}
	c.Data["json"] = resp
}

/**
4.试卷参数导入
*/

func (c *ApiController) InsertTopic() {

	defer c.ServeJSON()
	var req AddTopic
	var resp Response
	var err error

	err = json.Unmarshal(c.Ctx.Input.RequestBody, &req)
	if err != nil {
		log.Println(err)
		resp = Response{"10001", "cannot unmarshal", err}
		c.Data["json"] = resp
		return
	}
	// adminId := req.AdminId
	topicName := req.TopicName
	scoreType := req.ScoreType
	standardError := req.Error
	subjectName := req.SubjectName
	details := req.TopicDetails

	// ----------------------------------------------------
	// 添加subject
	var subject model.Subject
	subject.SubjectName = subjectName
	flag, err := model.GetSubjectBySubjectName(&subject, subjectName)
	if err != nil {
		log.Println(err)
	}
	subjectId := subject.SubjectId
	if !flag {
		err, subjectId = model.InsertSubject(&subject)
		if err != nil {
			log.Println(err)
			resp = Response{"30005", "科目导入错误  ", err}
			c.Data["json"] = resp
			return
		}
	}

	// 添加topic
	var topic model.Topic
	for _, detail := range req.TopicDetails {
		topic.QuestionScore += detail.DetailScore
	}
	topic.QuestionName = topicName
	topic.ScoreType = scoreType
	topic.StandardError = standardError
	topic.SubjectName = subjectName
	topic.ImportTime = util.GetCurrentTime()
	topic.SubjectId = subjectId
	topic.SelfScoreRate = req.SelfScoreRate

	err, questionId := model.InsertTopic(&topic)
	if err != nil {
		log.Println(err)
		resp = Response{"30006", " 大题参数导入错误 ", err}
		c.Data["json"] = resp
		return
	}

	var addTopicVO AddTopicVO
	var addTopicDetailVOList = make([]AddTopicDetailVO, len(details))

	for i := 0; i < len(details); i++ {
		var subTopic model.SubTopic
		subTopic.QuestionDetailName = details[i].TopicDetailName
		subTopic.QuestionDetailScore = details[i].DetailScore
		subTopic.ScoreType = details[i].DetailScoreTypes
		subTopic.QuestionId = questionId
		if err := model.InsertSubTopic(&subTopic); err != nil {
			log.Println(err)
			resp = Response{"30007", "小题参数导入错误  ", err}
			c.Data["json"] = resp
			return
		}
		addTopicDetailVOList[i].QuestionDetailId = subTopic.QuestionDetailId
	}
	addTopicVO.QuestionId = questionId
	addTopicVO.QuestionDetailIds = addTopicDetailVOList
	// ----------------------------------------------------
	data := make(map[string]interface{})
	data["addTopicVO"] = addTopicVO
	resp = Response{"10000", "OK", data}
	c.Data["json"] = resp

}

/**
5.科目选择
*/

func (c *ApiController) SubjectList() {
	defer c.ServeJSON()
	var resp Response

	// supervisorId := req.SupervisorId
	// ----------------------------------------------------
	// 获取科目列表
	subjects := make([]model.Subject, 0)
	err := model.FindSubjectList(&subjects)
	if err != nil {
		log.Println(err)
		resp = Response{"30008", "科目列表获取错误  ", err}
		c.Data["json"] = resp
		return
	}

	var subjectVOList = make([]SubjectListVO, len(subjects))
	for i := 0; i < len(subjects); i++ {
		subjectVOList[i].SubjectName = subjects[i].SubjectName
		subjectVOList[i].SubjectId = subjects[i].SubjectId
	}

	// ----------------------------------------------------
	data := make(map[string]interface{})
	data["subjectVOList"] = subjectVOList
	resp = Response{"10000", "OK", data}
	c.Data["json"] = resp
}

//
// /**
// 6.试卷分配界面
// */
// func (c *ApiController) DistributionInfo() {
//
// 	defer c.ServeJSON()
// 	var req DistributionInfo
// 	var resp Response
// 	var err error
//
// 	err = json.Unmarshal(c.Ctx.Input.RequestBody, &req)
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"10001", "cannot unmarshal", err}
// 		c.Data["json"] = resp
// 		return
// 	}
// 	// supervisorId := req.SupervisorId
// 	questionId := req.QuestionId
//
// 	// ----------------------------------------------------
// 	// 标注输出
// 	var distributionInfoVO DistributionInfoVO
// 	// 获取试卷导入数量
// 	var topic model.Topic
// 	topic.QuestionId = questionId
// 	err = topic.GetTopic(questionId)
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"30009", "获取试卷导入数量错误  ", err}
// 		c.Data["json"] = resp
// 		return
// 	}
//
// 	scoreType := topic.ScoreType
// 	distributionInfoVO.ScoreType = scoreType
//
// 	importNumber := topic.ImportNumber
// 	distributionInfoVO.ImportTestNumber = importNumber
// 	// 获取试卷未分配数量
// 	// 查询相应试卷
// 	papers := make([]model.TestPaper, 0)
// 	if err := model.FindUnDistributeTest(questionId, &papers); err != nil {
// 		log.Println(err)
// 		resp = Response{"30012", "试卷分配异常，无法获取未分配试卷 ", err}
// 		c.Data["json"] = resp
// 		return
// 	}
//
// 	distributionInfoVO.LeftTestNumber = len(papers)
// 	// 获取在线人数
//
// 	// 查找在线且未分配试卷的人
// 	usersList := make([]model.User, 0)
// 	if err := model.FindUsers(&usersList, topic.SubjectName); err != nil {
// 		log.Println(err)
// 		resp = Response{"30010", "获取可分配人数错误  ", err}
// 		c.Data["json"] = resp
// 		return
// 	}
// 	distributionInfoVO.OnlineNumber = int64(len(usersList))
//
// 	// ----------------------------------------------------
// 	data := make(map[string]interface{})
// 	data["distributionInfoVO"] = distributionInfoVO
// 	resp = Response{"10000", "OK", data}
// 	c.Data["json"] = resp
//
// }
//
// /**
// 7.试卷分配
// */
// func (c *ApiController) Distribution() {
//
// 	defer c.ServeJSON()
// 	var req Distribution
// 	var resp Response
// 	var err error
//
// 	err = json.Unmarshal(c.Ctx.Input.RequestBody, &req)
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"10001", "cannot unmarshal", err}
// 		c.Data["json"] = resp
// 		return
// 	}
// 	// supervisorId := req.SupervisorId
// 	questionId := req.QuestionId
// 	testNumber := req.TestNumber
// 	userNumber := req.UserNumber
// 	// ----------------------------------------------------
//
// 	// 是否需要二次阅卷
// 	var topic model.Topic
// 	topic.QuestionId = questionId
// 	err = topic.GetTopic(questionId)
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"30011", "试卷分配异常,无法获取试卷批改次数 ", err}
// 		c.Data["json"] = resp
// 		return
// 	}
// 	scoreType := topic.ScoreType
//
// 	// 查询相应试卷
// 	papers := make([]model.TestPaper, 0)
// 	err = model.FindUnDistributeTest(questionId, &papers)
//
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"30012", "试卷分配异常，无法获取未分配试卷 ", err}
// 		c.Data["json"] = resp
// 		return
// 	}
// 	testPapers := papers[:testNumber]
//
// 	// 查找在线且未分配试卷的人
// 	usersList := make([]model.User, 0)
// 	err = model.FindUsers(&usersList, topic.SubjectName)
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"30013", "试卷分配异常，无法获取可分配阅卷员 ", err}
// 		c.Data["json"] = resp
// 		return
// 	}
// 	users := usersList[:userNumber]
//
// 	// 第一次分配试卷
// 	countUser := make([]int, userNumber)
// 	var ii int
// 	for i := 0; i < len(testPapers); {
// 		ii = i
// 		for j := 0; j < len(users); j++ {
//
// 			// 修改testPaper改为已分配
// 			testPapers[ii].CorrectingNumber = 1
// 			err := testPapers[ii].Update()
// 			if err != nil {
// 				log.Println(err)
// 				resp = Response{"30014", "试卷第一次分配异常，无法更改试卷状态 ", err}
// 				c.Data["json"] = resp
// 				return
// 			}
//
// 			// 添加试卷未批改记录
// 			var underCorrectedPaper model.UnderCorrectedPaper
// 			underCorrectedPaper.TestIds = testPapers[ii].TestIds
// 			underCorrectedPaper.QuestionId = testPapers[ii].QuestionId
// 			underCorrectedPaper.TestQuestionType = 1
// 			underCorrectedPaper.UserId = users[j].UserId
// 			if err := underCorrectedPaper.Save(); err != nil {
// 				log.Println(err)
// 				resp = Response{"30015", "试卷第一次分配异常，无法生成待批改试卷 ", err}
// 				c.Data["json"] = resp
// 				return
// 			}
//
// 			countUser[j]++
// 			testNumber--
// 			ii++
//
// 		}
// 		i += userNumber
// 	}
//
// 	// 修改user变为已分配
// 	for _, user := range users {
// 		user.GetUser(user.UserId)
// 		fmt.Println("user: ", user)
//
// 		// user.IsDistribute = true
// 		user.QuestionId = questionId
// 		if err := user.UpdateCols("is_distribute", "question_id"); err != nil {
// 			log.Println(err)
// 			resp = Response{"30019", "试卷分配异常，用户分配状态更新失败 ", err}
// 			c.Data["json"] = resp
// 			return
// 		}
// 	}
//
// 	// 二次阅卷
// 	if scoreType == 2 {
// 		testNumber = len(testPapers)
// 		revers(users)
// 		var ii int
// 		for i := 0; i < len(testPapers); {
// 			ii = i
// 			for j := 0; j < len(users); j++ {
// 				if testNumber == 0 {
// 					break
// 				} else {
// 					// 修改testPaper改为已分配
// 					testPapers[ii].CorrectingNumber = 1
// 					err := testPapers[ii].Update()
// 					if err != nil {
// 						log.Println(err)
// 						resp = Response{"30016", "试卷第二次分配异常，无法更改试卷状态 ", err}
// 						c.Data["json"] = resp
// 						return
// 					}
//
// 					// 添加试卷未批改记录
// 					var underCorrectedPaper model.UnderCorrectedPaper
// 					underCorrectedPaper.TestIds = testPapers[ii].TestIds
// 					underCorrectedPaper.QuestionId = testPapers[ii].QuestionId
// 					underCorrectedPaper.TestQuestionType = 2
// 					underCorrectedPaper.UserId = users[j].UserId
// 					err = underCorrectedPaper.Save()
// 					if err != nil {
// 						log.Println(err)
// 						resp = Response{"30017", "试卷第二次分配异常，无法更改试卷状态 ", err}
// 						c.Data["json"] = resp
// 						return
// 					}
// 					countUser[j] = countUser[j] + 1
// 					testNumber--
// 					ii++
// 				}
// 			}
// 			i += userNumber
// 		}
//
// 	}
//
// 	for i := 0; i < userNumber; i++ {
// 		// 添加试卷分配表
// 		var paperDistribution model.PaperDistribution
// 		paperDistribution.TestDistributionNumber = int64(countUser[i])
// 		paperDistribution.UserId = users[i].UserId
// 		paperDistribution.QuestionId = questionId
// 		err := paperDistribution.Save()
// 		if err != nil {
// 			log.Println(err)
// 			resp = Response{"30018", "试卷分配异常，试卷分配添加异常 ", err}
// 			c.Data["json"] = resp
// 			return
// 		}
// 	}
//
// 	// ----------------------------------------------------
// 	data := make(map[string]interface{})
// 	data["data"] = nil
// 	resp = Response{"10000", "OK", data}
// 	c.Data["json"] = resp
//
// }

func (c *ApiController) TopicList() {
	defer c.ServeJSON()
	var resp Response
	// supervisorId := req.SupervisorId

	// ----------------------------------------------------
	// 获取大题列表
	topics := make([]model.Topic, 0)
	err := model.FindTopicList(&topics)
	if err != nil {
		log.Println(err)
		resp = Response{"30021", "获取大题参数设置记录表失败  ", err}
		c.Data["json"] = resp
		return
	}

	var topicVOList = make([]TopicVO, len(topics))
	for i := 0; i < len(topics); i++ {

		topicVOList[i].SubjectName = topics[i].SubjectName
		topicVOList[i].TopicName = topics[i].QuestionName
		topicVOList[i].Score = topics[i].QuestionScore
		topicVOList[i].StandardError = topics[i].StandardError
		topicVOList[i].ScoreType = topics[i].ScoreType
		topicVOList[i].TopicId = topics[i].QuestionId
		topicVOList[i].ImportTime = topics[i].ImportTime

		subTopics := make([]model.SubTopic, 0)
		model.FindSubTopicsByQuestionId(topics[i].QuestionId, &subTopics)
		if err != nil {
			log.Println(err)
			resp = Response{"30022", "获取小题参数设置记录表失败  ", err}
			c.Data["json"] = resp
			return
		}
		subTopicVOS := make([]SubTopicVO, len(subTopics))
		for j := 0; j < len(subTopics); j++ {
			subTopicVOS[j].SubTopicId = subTopics[j].QuestionDetailId
			subTopicVOS[j].SubTopicName = subTopics[j].QuestionDetailName
			subTopicVOS[j].Score = subTopics[j].QuestionDetailScore
			subTopicVOS[j].ScoreDistribution = subTopics[j].ScoreType
			subTopicVOS[j].IsSecondScore = subTopics[j].IsSecondScore
		}
		topicVOList[i].SubTopicVOList = subTopicVOS
	}

	// ----------------------------------------------------
	data := make(map[string]interface{})
	data["topicVOList"] = topicVOList
	resp = Response{"10000", "OK", data}
	c.Data["json"] = resp
}

// // DistributionRecord ...
// func (c *ApiController) DistributionRecord() {
// 	defer c.ServeJSON()
// 	var req DistributionRecord
// 	var resp Response
//
// 	err := json.Unmarshal(c.Ctx.Input.RequestBody, &req)
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"10001", "cannot unmarshal", err}
// 		c.Data["json"] = resp
// 		return
// 	}
// 	// supervisorId := req.SupervisorId
// 	subjectName := req.SubjectName
// 	// ----------------------------------------------------
// 	// 获取大题列表
// 	topics := make([]model.Topic, 0)
// 	err = model.FindTopicBySubNameList(&topics, subjectName)
// 	if err != nil {
// 		log.Println(err)
// 		resp = Response{"30023", "获取试卷分配记录表失败  ", err}
// 		c.Data["json"] = resp
// 		return
// 	}
//
// 	var distributionRecordList = make([]DistributionRecordVO, len(topics))
// 	for i := 0; i < len(topics); i++ {
//
// 		distributionRecordList[i].TopicId = topics[i].QuestionId
// 		distributionRecordList[i].TopicName = topics[i].QuestionName
// 		distributionRecordList[i].ImportNumber = topics[i].ImportNumber
// 		distributionTestNumber, err := model.CountTestDistributionNumberByQuestionId(topics[i].QuestionId)
// 		if err != nil {
// 			log.Println(err)
// 			resp = Response{"30024", "获取试卷分配记录表失败，统计试卷已分配数失败  ", err}
// 			c.Data["json"] = resp
// 			return
// 		}
// 		distributionUserNumber, err := model.CountUserDistributionNumberByQuestionId(topics[i].QuestionId)
// 		if err != nil {
// 			log.Println(err)
// 			resp = Response{"30025", "获取试卷分配记录表失败，统计用户已分配数失败  ", err}
// 			c.Data["json"] = resp
// 			return
// 		}
// 		distributionRecordList[i].DistributionUserNumber = distributionUserNumber
// 		distributionRecordList[i].DistributionTestNumber = distributionTestNumber
//
// 	}
//
// 	// ----------------------------------------------------
// 	data := make(map[string]interface{})
// 	data["distributionRecordList"] = distributionRecordList
// 	resp = Response{"10000", "OK", data}
// 	c.Data["json"] = resp
// }

/**
试卷删除
*/

func (c *ApiController) DeleteTest() {

	defer c.ServeJSON()
	var req DeleteTest
	var resp Response
	var err error

	err = json.Unmarshal(c.Ctx.Input.RequestBody, &req)
	if err != nil {
		log.Println(err)
		resp = Response{"10001", "cannot unmarshal", err}
		c.Data["json"] = resp
		return
	}
	// adminId := req.AdminId
	questionId := req.QuestionId

	// ----------------------------------------------------
	count, err := model.CountUnScoreTestNumberByQuestionId(questionId)
	if count == 0 {
		model.DeleteAllTest(questionId)
		subTopics := make([]model.SubTopic, 0)
		model.FindSubTopicsByQuestionId(questionId, &subTopics)
		for j := 0; j < len(subTopics); j++ {
			subTopic := subTopics[j]
			testPaperInfos := make([]model.TestPaperInfo, 0)
			model.FindTestPaperInfoByQuestionDetailId(subTopic.QuestionDetailId, &testPaperInfos)
			for k := 0; k < len(testPaperInfos); k++ {
				// picName := testPaperInfos[k].PicSrc
				// src := "./img/" + picName
				// os.Remove(src)
				testPaperInfos[k].Delete()
			}
		}

	} else {
		resp = Response{"30030", "试卷未批改完不能删除  ", err}
		c.Data["json"] = resp
		return
	}

	// ----------------------------------------------------
	data := make(map[string]interface{})
	data["data"] = nil
	resp = Response{"10000", "OK", data}
	c.Data["json"] = resp

}
