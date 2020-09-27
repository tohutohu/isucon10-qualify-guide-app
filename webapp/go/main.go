package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
	"golang.org/x/sync/errgroup"
)

const Limit = 20
const NazotteLimit = 50

var estateDb *sqlx.DB
var chairDb *sqlx.DB
var estateMySQLConnectionData MySQLConnectionEnv
var chairMySQLConnectionData MySQLConnectionEnv
var chairSearchCondition ChairSearchCondition
var estateSearchCondition EstateSearchCondition

type InitializeResponse struct {
	Language string `json:"language"`
}

type Chair struct {
	ID          int64  `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Description string `db:"description" json:"description"`
	Thumbnail   string `db:"thumbnail" json:"thumbnail"`
	Price       int64  `db:"price" json:"price"`
	Height      int64  `db:"height" json:"height"`
	Width       int64  `db:"width" json:"width"`
	Depth       int64  `db:"depth" json:"depth"`
	Color       string `db:"color" json:"color"`
	Features    string `db:"features" json:"features"`
	Kind        string `db:"kind" json:"kind"`
	Popularity  int64  `db:"popularity" json:"-"`
	Stock       int64  `db:"stock" json:"-"`
}

type ChairSearchResponse struct {
	Count  int64   `json:"count"`
	Chairs []Chair `json:"chairs"`
}

type ChairListResponse struct {
	Chairs []Chair `json:"chairs"`
}

//Estate 物件
type Estate struct {
	ID          int64   `db:"id" json:"id"`
	Thumbnail   string  `db:"thumbnail" json:"thumbnail"`
	Name        string  `db:"name" json:"name"`
	Description string  `db:"description" json:"description"`
	Latitude    float64 `db:"latitude" json:"latitude"`
	Longitude   float64 `db:"longitude" json:"longitude"`
	Address     string  `db:"address" json:"address"`
	Rent        int64   `db:"rent" json:"rent"`
	DoorHeight  int64   `db:"door_height" json:"doorHeight"`
	DoorWidth   int64   `db:"door_width" json:"doorWidth"`
	Features    string  `db:"features" json:"features"`
	Popularity  int64   `db:"popularity" json:"-"`
}

//EstateSearchResponse estate/searchへのレスポンスの形式
type EstateSearchResponse struct {
	Count   int64    `json:"count"`
	Estates []Estate `json:"estates"`
}

type EstateListResponse struct {
	Estates []Estate `json:"estates"`
}

type Coordinate struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Coordinates struct {
	Coordinates []Coordinate `json:"coordinates"`
}

type Range struct {
	ID  int64 `json:"id"`
	Min int64 `json:"min"`
	Max int64 `json:"max"`
}

type RangeCondition struct {
	Prefix string   `json:"prefix"`
	Suffix string   `json:"suffix"`
	Ranges []*Range `json:"ranges"`
}

type ListCondition struct {
	List []string `json:"list"`
}

type EstateSearchCondition struct {
	DoorWidth  RangeCondition `json:"doorWidth"`
	DoorHeight RangeCondition `json:"doorHeight"`
	Rent       RangeCondition `json:"rent"`
	Feature    ListCondition  `json:"feature"`
}

type ChairSearchCondition struct {
	Width   RangeCondition `json:"width"`
	Height  RangeCondition `json:"height"`
	Depth   RangeCondition `json:"depth"`
	Price   RangeCondition `json:"price"`
	Color   ListCondition  `json:"color"`
	Feature ListCondition  `json:"feature"`
	Kind    ListCondition  `json:"kind"`
}

type BoundingBox struct {
	// TopLeftCorner 緯度経度が共に最小値になるような点の情報を持っている
	TopLeftCorner Coordinate
	// BottomRightCorner 緯度経度が共に最大値になるような点の情報を持っている
	BottomRightCorner Coordinate
}

type MySQLConnectionEnv struct {
	Host     string
	Port     string
	User     string
	DBName   string
	Password string
}

type RecordMapper struct {
	Record []string

	offset int
	err    error
}

func (r *RecordMapper) next() (string, error) {
	if r.err != nil {
		return "", r.err
	}
	if r.offset >= len(r.Record) {
		r.err = fmt.Errorf("too many read")
		return "", r.err
	}
	s := r.Record[r.offset]
	r.offset++
	return s, nil
}

func (r *RecordMapper) NextInt() int {
	s, err := r.next()
	if err != nil {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		r.err = err
		return 0
	}
	return i
}

func (r *RecordMapper) NextFloat() float64 {
	s, err := r.next()
	if err != nil {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		r.err = err
		return 0
	}
	return f
}

func (r *RecordMapper) NextString() string {
	s, err := r.next()
	if err != nil {
		return ""
	}
	return s
}

func (r *RecordMapper) Err() error {
	return r.err
}

func NewEstateMySQLConnectionEnv() MySQLConnectionEnv {
	return MySQLConnectionEnv{
		Host:     getEnv("MYSQL_ESTATE_HOST", "127.0.0.1"),
		Port:     getEnv("MYSQL_ESTATE_PORT", "3306"),
		User:     getEnv("MYSQL_ESTATE_USER", "isucon"),
		DBName:   getEnv("MYSQL_ESTATE_DBNAME", "isuumo"),
		Password: getEnv("MYSQL_ESTATE_PASS", "isucon"),
	}
}

func NewChairMySQLConnectionEnv() MySQLConnectionEnv {
	return MySQLConnectionEnv{
		Host:     getEnv("MYSQL_CHAIR_HOST", "127.0.0.1"),
		Port:     getEnv("MYSQL_CHAIR_PORT", "3306"),
		User:     getEnv("MYSQL_CHAIR_USER", "isucon"),
		DBName:   getEnv("MYSQL_CHAIR_DBNAME", "isuumo"),
		Password: getEnv("MYSQL_CHAIR_PASS", "isucon"),
	}
}

func getEnv(key, defaultValue string) string {
	val := os.Getenv(key)
	if val != "" {
		return val
	}
	return defaultValue
}

//ConnectDB isuumoデータベースに接続する
func (mc *MySQLConnectionEnv) ConnectDB() (*sqlx.DB, error) {
	dsn := strings.Join([]string{mc.User, ":", mc.Password, "@tcp(", mc.Host, ":", mc.Port, ")/", mc.DBName}, "")
	return sqlx.Open("mysql", dsn)
}

func init() {
	jsonText, err := ioutil.ReadFile("../fixture/chair_condition.json")
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	json.Unmarshal(jsonText, &chairSearchCondition)

	jsonText, err = ioutil.ReadFile("../fixture/estate_condition.json")
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	json.Unmarshal(jsonText, &estateSearchCondition)

	recommendCacheMux = sync.RWMutex{}
	reset()
}

var estatesPool = sync.Pool{
	New: func() interface{} {
		return make([]Estate, 0, 100)
	},
}

func putEstatesPool(estates []Estate) {
	estates = estates[:0]
	estatesPool.Put(estates)
}

var IDsPool = sync.Pool{
	New: func() interface{} {
		return make([]int64, 0, 100)
	},
}

func putIDsPool(estateIDs []int64) {
	estateIDs = estateIDs[:0]
	IDsPool.Put(estateIDs)
}

var chairsPool = sync.Pool{
	New: func() interface{} {
		return make([]Chair, 0, 100)
	},
}

func putChairsPool(chairs []Chair) {
	chairs = chairs[:0]
	chairsPool.Put(chairs)
}

var estateMap sync.Map
var chairMap sync.Map

func reset() {
	resetChair()
	resetLowPriced()
	estateMap = sync.Map{}
	chairMap = sync.Map{}
}

var lowPriced sync.Map

func resetLowPriced() {
	lowPriced = sync.Map{}
}

var recommendCache map[int]EstateListResponse
var recommendCacheMux sync.RWMutex

func resetChair() {
	recommendCache = make(map[int]EstateListResponse)
}

func main() {
	// Echo instance
	e := echo.New()
	// e.Debug = true
	// e.Logger.SetLevel(log.DEBUG)

	// Middleware
	// e.Use(middleware.Logger())
	// e.Use(middleware.Recover())

	// Initialize
	e.POST("/initialize", initialize)

	// Chair Handler
	e.GET("/api/chair/:id", getChairDetail)
	e.POST("/api/chair", postChair)
	e.GET("/api/chair/search", searchChairs)
	e.GET("/api/chair/low_priced", getLowPricedChair)
	e.GET("/api/chair/search/condition", getChairSearchCondition)
	e.POST("/api/chair/buy/:id", buyChair)

	// Estate Handler
	e.GET("/api/estate/:id", getEstateDetail)
	e.POST("/api/estate", postEstate)
	e.GET("/api/estate/search", searchEstates)
	e.GET("/api/estate/low_priced", getLowPricedEstate)
	e.POST("/api/estate/req_doc/:id", postEstateRequestDocument)
	e.POST("/api/estate/nazotte", searchEstateNazotte)
	e.GET("/api/estate/search/condition", getEstateSearchCondition)
	e.GET("/api/recommended_estate/:id", searchRecommendedEstateWithChair)

	estateMySQLConnectionData = NewEstateMySQLConnectionEnv()
	chairMySQLConnectionData = NewChairMySQLConnectionEnv()

	var err error
	estateDb, err = estateMySQLConnectionData.ConnectDB()
	if err != nil {
		e.Logger.Fatalf("DB connection failed : %v", err)
	}
	estateDb.SetMaxOpenConns(200)
	estateDb.SetMaxIdleConns(200)
	defer estateDb.Close()

	chairDb, err = chairMySQLConnectionData.ConnectDB()
	if err != nil {
		e.Logger.Fatalf("DB connection failed : %v", err)
	}
	chairDb.SetMaxOpenConns(200)
	chairDb.SetMaxIdleConns(200)
	defer chairDb.Close()

	// ここからソケット接続設定 ---
	socket_file := "/var/run/app.sock"
	os.Remove(socket_file)

	l, err := net.Listen("unix", socket_file)
	if err != nil {
		e.Logger.Fatal(err)
	}

	// go runユーザとnginxのユーザ（グループ）を同じにすれば777じゃなくてok
	err = os.Chmod(socket_file, 0777)
	if err != nil {
		e.Logger.Fatal(err)
	}

	e.Listener = l
	// ここまで ---
	// Start server
	// serverPort := fmt.Sprintf(":%v", getEnv("SERVER_PORT", "1323"))
	e.Logger.Fatal(e.Start(""))
}

func initialize(c echo.Context) error {
	reset()
	sqlDir := filepath.Join("..", "mysql", "db")
	estatePaths := []string{
		filepath.Join(sqlDir, "0_Schema.sql"),
		filepath.Join(sqlDir, "1_DummyEstateData.sql"),
	}

	chairPaths := []string{
		filepath.Join(sqlDir, "0_Schema.sql"),
		filepath.Join(sqlDir, "2_DummyChairData.sql"),
	}

	eg := errgroup.Group{}
	eg.Go(func() error {
		for _, p := range estatePaths {
			sqlFile, _ := filepath.Abs(p)
			cmdStr := fmt.Sprintf("mysql -h %v -u %v -p%v -P %v %v < %v",
				estateMySQLConnectionData.Host,
				estateMySQLConnectionData.User,
				estateMySQLConnectionData.Password,
				estateMySQLConnectionData.Port,
				estateMySQLConnectionData.DBName,
				sqlFile,
			)
			if err := exec.Command("bash", "-c", cmdStr).Run(); err != nil {
				return err
			}
		}
		return nil
	})
	eg.Go(func() error {
		for _, p := range chairPaths {
			sqlFile, _ := filepath.Abs(p)
			cmdStr := fmt.Sprintf("mysql -h %v -u %v -p%v -P %v %v < %v",
				chairMySQLConnectionData.Host,
				chairMySQLConnectionData.User,
				chairMySQLConnectionData.Password,
				chairMySQLConnectionData.Port,
				chairMySQLConnectionData.DBName,
				sqlFile,
			)
			if err := exec.Command("bash", "-c", cmdStr).Run(); err != nil {
				return err
			}
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	var estates []Estate
	query := `SELECT id,name,description,thumbnail,address,latitude,longitude,rent,door_height,door_width,features,popularity FROM estate`
	err := estateDb.Select(&estates, query)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	for _, estate := range estates {
		estateMap.Store(estate.ID, estate)
	}

	var chairs []Chair
	query = `SELECT id,name,description,thumbnail,price,height,width,depth,color,features,kind,popularity,stock FROM chair`
	err = chairDb.Select(&chairs, query)
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	for _, chair := range chairs {
		chairMap.Store(chair.ID, chair)
	}

	return c.JSON(http.StatusOK, InitializeResponse{
		Language: "go",
	})
}

var chairPool = sync.Pool{
	New: func() interface{} {
		return Chair{}
	},
}

func getChairDetail(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if val, ok := chairMap.Load(int64(id)); ok {
		if (val.(Chair)).Stock == 0 {
			return c.NoContent(http.StatusNotFound)
		}
		return c.JSON(http.StatusOK, val)
	}
	return c.NoContent(http.StatusNotFound)
}

func postChair(c echo.Context) error {
	header, err := c.FormFile("chairs")
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	f, err := header.Open()
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.LazyQuotes = false
	reader.ReuseRecord = true
	reader.FieldsPerRecord = 13
	records, err := reader.ReadAll()
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	query := strings.Builder{}
	query.WriteString("INSERT INTO chair(id,name,description,thumbnail,price,height,width,depth,color,features,kind,popularity,stock) VALUES")
	for idx, row := range records {
		rm := RecordMapper{Record: row}
		id := rm.NextInt()
		name := rm.NextString()
		description := rm.NextString()
		thumbnail := rm.NextString()
		price := rm.NextInt()
		height := rm.NextInt()
		width := rm.NextInt()
		depth := rm.NextInt()
		color := rm.NextString()
		features := rm.NextString()
		kind := rm.NextString()
		popularity := rm.NextInt()
		stock := rm.NextInt()
		if err := rm.Err(); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		if idx > 0 {
			query.WriteString(",")
		}
		query.WriteString(fmt.Sprintf(`(%d,"%s","%s","%s",%d,%d,%d,%d,"%s","%s","%s",%d,%d)`, id, name, description, thumbnail, price, height, width, depth, color, features, kind, popularity, stock))

		chairMap.Store(int64(id), Chair{
			ID:          int64(id),
			Name:        name,
			Description: description,
			Thumbnail:   thumbnail,
			Price:       int64(price),
			Height:      int64(height),
			Width:       int64(width),
			Depth:       int64(depth),
			Color:       color,
			Features:    features,
			Kind:        kind,
			Popularity:  int64(popularity),
			Stock:       int64(stock),
		})
	}
	_, err = chairDb.Exec(query.String())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	resetChair()
	lowPriced.Delete("chair")
	return c.NoContent(http.StatusCreated)
}

func searchChairs(c echo.Context) error {
	queryCondition := builderPool.Get().(*strings.Builder)
	defer putBuilderPool(queryCondition)

	if c.QueryParam("priceRangeId") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("p=")
		queryCondition.WriteString(c.QueryParam("priceRangeId"))
	}

	if c.QueryParam("heightRangeId") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("h=")
		queryCondition.WriteString(c.QueryParam("heightRangeId"))
	}

	if c.QueryParam("widthRangeId") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("w=")
		queryCondition.WriteString(c.QueryParam("widthRangeId"))
	}

	if c.QueryParam("depthRangeId") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("d=")
		queryCondition.WriteString(c.QueryParam("depthRangeId"))
	}

	if c.QueryParam("kind") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("kind='")
		queryCondition.WriteString(c.QueryParam("kind"))
		queryCondition.WriteString("'")
	}

	if c.QueryParam("color") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("color='")
		queryCondition.WriteString(c.QueryParam("color"))
		queryCondition.WriteString("'")
	}

	if c.QueryParam("features") != "" {
		for _, f := range strings.Split(c.QueryParam("features"), ",") {
			if queryCondition.Len() > 0 {
				queryCondition.WriteString(" AND ")
			}
			queryCondition.WriteString("FIND_IN_SET('")
			queryCondition.WriteString(f)
			queryCondition.WriteString("',f)>0")
		}
	}

	if queryCondition.Len() == 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	queryCondition.WriteString(" AND stock>0 ")

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	perPage, err := strconv.Atoi(c.QueryParam("perPage"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	searchQuery := "SELECT id FROM chair WHERE "
	countQuery := "SELECT COUNT(*) FROM chair WHERE "
	limitOffset := fmt.Sprintf(" ORDER BY popularity DESC, id ASC LIMIT %d OFFSET %d", perPage, page*perPage)

	var res ChairSearchResponse
	err = chairDb.Get(&res.Count, countQuery+queryCondition.String())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	chairIDs := IDsPool.Get().([]int64)
	defer putIDsPool(chairIDs)
	err = chairDb.Select(&chairIDs, searchQuery+queryCondition.String()+limitOffset)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, ChairSearchResponse{Count: 0, Chairs: []Chair{}})
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	chairs := chairsPool.Get().([]Chair)
	defer putChairsPool(chairs)
	for _, id := range chairIDs {
		val, _ := chairMap.Load(id)
		chairs = append(chairs, val.(Chair))
	}

	res.Chairs = chairs

	return c.JSON(http.StatusOK, res)
}

func buyChair(c echo.Context) error {
	m := echo.Map{}
	if err := c.Bind(&m); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	_, ok := m["email"].(string)
	if !ok {
		return c.NoContent(http.StatusBadRequest)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	_chair, ok := chairMap.Load(int64(id))
	if !ok {
		return c.NoContent(http.StatusNotFound)
	}
	chair := _chair.(Chair)
	chair.Stock--
	chairMap.Store(int64(id), chair)

	_, err = chairDb.Exec(fmt.Sprintf("UPDATE chair SET stock=stock-1 WHERE id=%s", c.Param("id")))
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	lowPriced.Delete("chair")

	return c.NoContent(http.StatusOK)
}

func getChairSearchCondition(c echo.Context) error {
	return c.JSON(http.StatusOK, chairSearchCondition)
}

func getLowPricedChair(c echo.Context) error {
	if val, ok := lowPriced.Load("chair"); ok {
		return c.JSON(http.StatusOK, ChairListResponse{Chairs: val.([]Chair)})
	}
	chairIDs := IDsPool.Get().([]int64)
	defer putIDsPool(chairIDs)
	query := `SELECT id FROM chair WHERE stock > 0 ORDER BY price ASC, id ASC LIMIT 20`
	err := chairDb.Select(&chairIDs, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, ChairListResponse{[]Chair{}})
		}
		return c.NoContent(http.StatusInternalServerError)
	}
	chairs := make([]Chair, len(chairIDs))
	for idx, id := range chairIDs {
		val, _ := chairMap.Load(id)
		chairs[idx] = val.(Chair)
	}

	lowPriced.Store("chair", chairs)
	return c.JSON(http.StatusOK, ChairListResponse{Chairs: chairs})
}

func getEstateDetail(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	if val, ok := estateMap.Load(int64(id)); ok {
		return c.JSON(http.StatusOK, val)
	}
	return c.NoContent(http.StatusNotFound)
}

func postEstate(c echo.Context) error {
	header, err := c.FormFile("estates")
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}
	f, err := header.Open()
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.LazyQuotes = false
	reader.ReuseRecord = true
	reader.FieldsPerRecord = 12
	records, err := reader.ReadAll()
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	query := strings.Builder{}
	query.WriteString("INSERT INTO estate(id,name,description,thumbnail,address,latitude,longitude,rent,door_height,door_width,features,popularity) VALUES")
	for idx, row := range records {
		rm := RecordMapper{Record: row}
		id := rm.NextInt()
		name := rm.NextString()
		description := rm.NextString()
		thumbnail := rm.NextString()
		address := rm.NextString()
		latitude := rm.NextFloat()
		longitude := rm.NextFloat()
		rent := rm.NextInt()
		doorHeight := rm.NextInt()
		doorWidth := rm.NextInt()
		features := rm.NextString()
		popularity := rm.NextInt()
		if err := rm.Err(); err != nil {
			return c.NoContent(http.StatusBadRequest)
		}
		if idx > 0 {
			query.WriteString(",")
		}
		query.WriteString(fmt.Sprintf(`(%d,"%s","%s","%s","%s",%f,%f,%d,%d,%d,"%s",%d)`, id, name, description, thumbnail, address, latitude, longitude, rent, doorHeight, doorWidth, features, popularity))

		estateMap.Store(int64(id), Estate{
			ID:          int64(id),
			Name:        name,
			Description: description,
			Thumbnail:   thumbnail,
			Address:     address,
			Latitude:    latitude,
			Longitude:   longitude,
			Rent:        int64(rent),
			DoorHeight:  int64(doorHeight),
			DoorWidth:   int64(doorWidth),
			Features:    features,
			Popularity:  int64(popularity),
		})
	}
	_, err = estateDb.Exec(query.String())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	lowPriced.Delete("estate")
	return c.NoContent(http.StatusCreated)
}

var paramsPool = sync.Pool{
	New: func() interface{} {
		return make([]interface{}, 0, 20)
	},
}

func putParamsPool(params []interface{}) {
	params = params[:0]
	paramsPool.Put(params)
}

var conditionsPool = sync.Pool{
	New: func() interface{} {
		return make([]string, 0, 20)
	},
}

func putConditionsPool(conditions []string) {
	conditions = conditions[:0]
	conditionsPool.Put(conditions)
}

func searchEstates(c echo.Context) error {
	queryCondition := builderPool.Get().(*strings.Builder)
	defer putBuilderPool(queryCondition)
	if c.QueryParam("doorHeightRangeId") != "" {
		queryCondition.WriteString("h=")
		queryCondition.WriteString(c.QueryParam("doorHeightRangeId"))
	}

	if c.QueryParam("doorWidthRangeId") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("w=")
		queryCondition.WriteString(c.QueryParam("doorWidthRangeId"))
	}

	if c.QueryParam("rentRangeId") != "" {
		if queryCondition.Len() > 0 {
			queryCondition.WriteString(" AND ")
		}
		queryCondition.WriteString("r=")
		queryCondition.WriteString(c.QueryParam("rentRangeId"))
	}

	if c.QueryParam("features") != "" {
		for _, f := range strings.Split(c.QueryParam("features"), ",") {
			if queryCondition.Len() > 0 {
				queryCondition.WriteString(" AND ")
			}
			queryCondition.WriteString("FIND_IN_SET('")
			queryCondition.WriteString(f)
			queryCondition.WriteString("',f)>0")
		}
	}

	if queryCondition.Len() == 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	page, err := strconv.Atoi(c.QueryParam("page"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	perPage, err := strconv.Atoi(c.QueryParam("perPage"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	searchQuery := "SELECT id FROM estate WHERE "
	countQuery := "SELECT COUNT(*) FROM estate WHERE "
	limitOffset := fmt.Sprintf(" ORDER BY popularity DESC, id ASC LIMIT %d OFFSET %d", perPage, page*perPage)

	var res EstateSearchResponse
	err = estateDb.Get(&res.Count, countQuery+queryCondition.String())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	estateIDs := IDsPool.Get().([]int64)
	defer putIDsPool(estateIDs)
	err = estateDb.Select(&estateIDs, searchQuery+queryCondition.String()+limitOffset)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, EstateSearchResponse{Count: 0, Estates: []Estate{}})
		}
		return c.NoContent(http.StatusInternalServerError)
	}
	estates := estatesPool.Get().([]Estate)
	defer putEstatesPool(estates)
	for _, id := range estateIDs {
		val, _ := estateMap.Load(id)
		estates = append(estates, val.(Estate))
	}

	res.Estates = estates

	return c.JSON(http.StatusOK, res)
}

func getLowPricedEstate(c echo.Context) error {
	if val, ok := lowPriced.Load("estate"); ok {
		return c.JSON(http.StatusOK, EstateListResponse{Estates: val.([]Estate)})
	}
	estateIDs := IDsPool.Get().([]int64)
	defer putIDsPool(estateIDs)
	query := `SELECT id FROM estate ORDER BY rent ASC, id ASC LIMIT 20`
	err := estateDb.Select(&estateIDs, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, EstateListResponse{[]Estate{}})
		}
		return c.NoContent(http.StatusInternalServerError)
	}
	estates := make([]Estate, len(estateIDs))
	for idx, id := range estateIDs {
		val, _ := estateMap.Load(id)
		estates[idx] = val.(Estate)
	}

	lowPriced.Store("estate", estates)

	return c.JSON(http.StatusOK, EstateListResponse{Estates: estates})
}

func searchRecommendedEstateWithChair(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	_chair, _ := chairMap.Load(int64(id))
	chair := _chair.(Chair)

	estateIDs := IDsPool.Get().([]int64)
	defer putIDsPool(estateIDs)
	w := chair.Width
	h := chair.Height
	d := chair.Depth
	if w > h {
		w, h = h, w
	}
	if h > d {
		h, d = d, h
	}
	query := fmt.Sprintf(`SELECT id FROM estate WHERE (door_width>=%d AND door_height>=%d) OR (door_width>=%d AND door_height>=%d) ORDER BY popularity DESC, id ASC LIMIT 20`, w, h, h, w)
	err = estateDb.Select(&estateIDs, query)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusOK, EstateListResponse{[]Estate{}})
		}
		return c.NoContent(http.StatusInternalServerError)
	}
	estates := estatesPool.Get().([]Estate)
	defer putEstatesPool(estates)
	for _, id := range estateIDs {
		val, _ := estateMap.Load(id)
		estates = append(estates, val.(Estate))
	}

	return c.JSON(http.StatusOK, EstateListResponse{estates})
}

var emptyEstateSearchResponse = EstateSearchResponse{Count: 0, Estates: []Estate{}}
var coordinatePool = sync.Pool{
	New: func() interface{} {
		return Coordinates{}
	},
}

var estateSearchResponsePool = sync.Pool{
	New: func() interface{} {
		return EstateSearchResponse{}
	},
}

func searchEstateNazotte(c echo.Context) error {
	coordinates := coordinatePool.Get().(Coordinates)
	err := c.Bind(&coordinates)
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if len(coordinates.Coordinates) == 0 {
		return c.NoContent(http.StatusBadRequest)
	}

	b := coordinates.getBoundingBox()
	estateIDs := IDsPool.Get().([]int64)
	defer putIDsPool(estateIDs)
	txt := builderPool.Get().(*strings.Builder)
	defer putBuilderPool(txt)
	txt.WriteString("'POLYGON((")
	for idx, c := range coordinates.Coordinates {
		if idx > 0 {
			txt.WriteRune(',')
		}
		txt.WriteString(fmt.Sprintf("%f %f", c.Latitude, c.Longitude))
	}
	txt.WriteString("))'")

	query := fmt.Sprintf(`SELECT id FROM estate WHERE latitude<=%f AND latitude>=%f AND longitude<=%f AND longitude>=%f AND ST_Contains(ST_PolygonFromText(%s),l) ORDER BY popularity DESC, id ASC LIMIT 50`, b.BottomRightCorner.Latitude, b.TopLeftCorner.Latitude, b.BottomRightCorner.Longitude, b.TopLeftCorner.Latitude, txt.String())
	err = estateDb.Select(&estateIDs, query)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusOK, emptyEstateSearchResponse)
	} else if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	re := estateSearchResponsePool.Get().(EstateSearchResponse)
	defer estateSearchResponsePool.Put(re)
	re.Estates = estatesPool.Get().([]Estate)
	defer putEstatesPool(re.Estates)
	for _, id := range estateIDs {
		val, _ := estateMap.Load(id)
		re.Estates = append(re.Estates, val.(Estate))
	}
	re.Count = int64(len(re.Estates))

	return c.JSON(http.StatusOK, re)
}

var mapPool = sync.Pool{
	New: func() interface{} {
		return echo.Map{}
	},
}

var estatePool = sync.Pool{
	New: func() interface{} {
		return Estate{}
	},
}

func postEstateRequestDocument(c echo.Context) error {
	m := mapPool.Get().(echo.Map)
	defer mapPool.Put(m)
	if err := c.Bind(&m); err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	_, ok := m["email"].(string)
	if !ok {
		return c.NoContent(http.StatusBadRequest)
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.NoContent(http.StatusBadRequest)
	}

	if _, ok := estateMap.Load(int64(id)); ok {
		return c.NoContent(http.StatusOK)
	}
	return c.NoContent(http.StatusNotFound)
}

func getEstateSearchCondition(c echo.Context) error {
	return c.JSON(http.StatusOK, estateSearchCondition)
}

func (cs Coordinates) getBoundingBox() BoundingBox {
	coordinates := cs.Coordinates
	boundingBox := BoundingBox{
		TopLeftCorner: Coordinate{
			Latitude: coordinates[0].Latitude, Longitude: coordinates[0].Longitude,
		},
		BottomRightCorner: Coordinate{
			Latitude: coordinates[0].Latitude, Longitude: coordinates[0].Longitude,
		},
	}
	for _, coordinate := range coordinates {
		if boundingBox.TopLeftCorner.Latitude > coordinate.Latitude {
			boundingBox.TopLeftCorner.Latitude = coordinate.Latitude
		}
		if boundingBox.TopLeftCorner.Longitude > coordinate.Longitude {
			boundingBox.TopLeftCorner.Longitude = coordinate.Longitude
		}

		if boundingBox.BottomRightCorner.Latitude < coordinate.Latitude {
			boundingBox.BottomRightCorner.Latitude = coordinate.Latitude
		}
		if boundingBox.BottomRightCorner.Longitude < coordinate.Longitude {
			boundingBox.BottomRightCorner.Longitude = coordinate.Longitude
		}
	}
	return boundingBox
}

var builderPool = sync.Pool{
	New: func() interface{} {
		builder := &strings.Builder{}
		builder.Grow(1024 * 1024)
		return builder
	},
}

func putBuilderPool(builder *strings.Builder) {
	builder.Reset()
	builderPool.Put(builder)
}
