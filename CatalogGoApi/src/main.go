package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/microsoft/go-mssqldb"
)

var db *sql.DB

type Catalog struct {
	Id           int64  `json: "id"`
	Name         string `json: "name"`
	Description  string `json: "description"`
	ReleasedDate string `json: "releasedDate"`
}

func main() {

	r := gin.Default()

	r.GET("/", getWelcomeMessage)

	r.GET("/catalogs", getCatalogs)
	r.GET("/catalogs/:id", getCatalogById)
	r.POST("/catalogs", addCatalog)
	r.PATCH("/catalogs", updateCatalog)
	r.DELETE("/catalogs", deleteCatalog)

	r.Run(":8080")
}

func initDatabase() (bool, error) {

	// Read secret from secret-volume
	//content, ex := ioutil.ReadFile("/etc/secret-volume/password")

	//fmt.Printf("--> Secret from mount volume, password: %s, error: %s", content, ex.Error())

	// read secret from Kubernetes secret store exposed in environment variable of the pod
	connString := os.ExpandEnv("server=$SECRET_DB_SERVER; user id=$SECRET_USERNAME; password=$SECRET_PASSWORD; port=1433; database=CatalogueDb")
	var err error

	// Create connection pool
	db, err = sql.Open("sqlserver", connString)
	if err != nil {
		log.Fatal("-->Error creating connection pool: ", err.Error())
		return false, err
	}
	ctx := context.Background()
	err = db.PingContext(ctx)
	if err != nil {
		log.Fatal(err.Error())
		return false, err
	}
	fmt.Printf("-->Database has been Connected!\n")

	return true, nil
}

func getWelcomeMessage(c *gin.Context) {
	c.String(http.StatusOK, "Catalog api service.")
}

func getCatalogs(c *gin.Context) {

	data, err := readCatalogues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
	} else {
		c.JSON(http.StatusOK, data)
	}
}

func getCatalogById(c *gin.Context) {

	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	catalogs, err := readCatalogues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
	} else {

		for _, catalog := range catalogs {

			if catalog.Id == id {
				c.JSON(http.StatusOK, catalog)
				return
			}
		}

		c.JSON(http.StatusNotFound, nil)
	}
}

func addCatalog(c *gin.Context) {
	var catalog Catalog
	err := c.BindJSON(&catalog)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	catalog.Id, err = createCatalog(catalog)

	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, catalog)
}

func updateCatalog(c *gin.Context) {
	var catalog Catalog
	err := c.BindJSON(&catalog)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}

	catalogs, err := readCatalogues()
	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	var found bool = false

	for _, c := range catalogs {

		if catalog.Id == c.Id {
			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, "Catalog does not exist.")
		return
	}

	_, err = updateDbCatalog(catalog)

	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusAccepted, catalog)

}

// CreateCatalog inserts an Catalog record
func createCatalog(catalog Catalog) (int64, error) {
	var err error

	ctx := context.Background()
	if db == nil {
		_, err = initDatabase()
	}

	// Check if database is alive.
	err = db.PingContext(ctx)
	if err != nil {
		db = nil
		return -1, err
	}

	tsql := `
      INSERT INTO CatalogueDb.dbo.Catalogues (Name, Description, ReleasedDate) VALUES (@Name, @Description, @ReleasedDate);
      select isNull(SCOPE_IDENTITY(), -1);
    `

	stmt, err := db.Prepare(tsql)
	if err != nil {
		return -1, err
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(
		ctx,
		sql.Named("Name", catalog.Name),
		sql.Named("Description", catalog.Description),
		sql.Named("ReleasedDate", catalog.ReleasedDate))
	var newID int64
	err = row.Scan(&newID)
	if err != nil {
		return -1, err
	}

	return newID, nil
}

// ReadCatalogues reads all catalog records
func readCatalogues() ([]Catalog, error) {
	var err error

	var catalogs = []Catalog{}

	ctx := context.Background()
	if db == nil {
		_, err = initDatabase()
	}

	// Check if database is alive.
	err = db.PingContext(ctx)
	if err != nil {
		db = nil
		return catalogs, err
	}

	tsql := fmt.Sprintf("SELECT Id, Name, Description, ReleasedDate FROM  CatalogueDb.dbo.Catalogues;")

	// Execute query
	rows, err := db.QueryContext(ctx, tsql)
	if err != nil {
		return catalogs, err
	}

	defer rows.Close()

	var count int

	// Iterate through the result set.
	for rows.Next() {
		var name, description, releasedDate string
		var id int64

		// Get values from row.
		err := rows.Scan(&id, &name, &description, &releasedDate)
		if err != nil {
			return catalogs, err
		}

		fmt.Printf("ID: %d, Name: %s, Description: %s, ReleasedDate: %s\n", id, name, description, releasedDate)
		catalogs = append(catalogs, Catalog{Id: id, Name: name, Description: description, ReleasedDate: releasedDate})
		count++
	}

	return catalogs, nil
}

// UpdateCatalog updates an Catalog's information
func updateDbCatalog(catalog Catalog) (int64, error) {
	var err error
	ctx := context.Background()
	if db == nil {
		_, err = initDatabase()
	}

	// Check if database is alive.
	err = db.PingContext(ctx)
	if err != nil {
		db = nil
		return -1, err
	}

	tsql := fmt.Sprintf("UPDATE CatalogueDb.dbo.Catalogues SET Description = @DEscription, Name = @Name, ReleasedDate = @ReleasedDate WHERE Id = @Id")

	// Execute non-query with named parameters
	result, err := db.ExecContext(
		ctx,
		tsql,
		sql.Named("Description", catalog.Description),
		sql.Named("Name", catalog.Name),
		sql.Named("ReleasedDate", catalog.ReleasedDate),
		sql.Named("Id", catalog.Id))
	if err != nil {
		return -1, err
	}

	return result.RowsAffected()
}

func deleteCatalog(c *gin.Context) {

	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	_, err := deleteDbCatalog(id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

// DeleteCatalog deletes an Catalog from the database
func deleteDbCatalog(id int64) (int64, error) {
	var err error
	ctx := context.Background()
	if db == nil {
		_, err = initDatabase()
	}

	// Check if database is alive.
	err = db.PingContext(ctx)
	if err != nil {
		db = nil
		return -1, err
	}

	tsql := fmt.Sprintf("DELETE FROM CatalogueDb.dbo.Catalogues WHERE Id = @Id;")

	// Execute non-query with named parameters
	result, err := db.ExecContext(ctx, tsql, sql.Named("Id", id))
	if err != nil {
		return -1, err
	}

	return result.RowsAffected()
}
