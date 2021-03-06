package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	bolt "go.etcd.io/bbolt"
	"log"
	"net/http"
	"os"
	"strings"
)

type Server struct {
	DB *bolt.DB
}

func NewServer(path string) (*Server, error) {
	db, err := NewDB(path)
	if err != nil {
		return nil, err
	}
	return &Server{DB: db}, nil
}

func NewDB(path string) (*bolt.DB, error) {
	db, err := bolt.Open(path, 0666, nil)
	if err != nil {
		return nil, err
	}
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("links"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	return db, nil
}

func main() {
	s, err := NewServer("my.db")
	if err != nil {
		panic(err)
	}
	defer s.DB.Close()

	router := gin.Default()
	router.StaticFile("/favicon.ico", "./favicon.ico")
	router.LoadHTMLFiles("index.html")
	router.GET("/:dest", s.redirect)
	router.GET("/", s.new)
	router.POST("/", s.create)

	PORT := os.Getenv("PORT")
	if PORT == "" {
		PORT = "8080"
	}

	router.Run(":" + PORT)
}

func (s *Server) redirect(c *gin.Context) {
	if c.Param("dest") == "links" {
		s.DB.View(func(tx *bolt.Tx) error {
			// Assume bucket exists and has keys
			links := []string{}
			b := tx.Bucket([]byte("links"))

			cur := b.Cursor()

			for k, v := cur.First(); k != nil; k, v = cur.Next() {
				links = append(links, fmt.Sprintf("%s | %s", k, v))
			}

			c.IndentedJSON(http.StatusOK, gin.H{"links": links})
			return nil
		})
		return
	}

	var redir string
	if err := s.DB.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte("links")).Get([]byte(c.Param("dest")))
		fmt.Printf("LINK %s | DEST %s\n", c.Param("dest"), v)
		redir = string(v)
		return nil
	}); err != nil {
		log.Fatal(err)
	}

	if redir != "" {
		c.Redirect(http.StatusSeeOther, redir)
	} else {
		//output := fmt.Sprintf("%s does not exist", redir)
		c.HTML(http.StatusOK, "index.html", "index.html")
	}
}

func (s *Server) new(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", "index.html")
}

func (s *Server) create(c *gin.Context) {
	type Request struct {
		Link string
		Dest string
	}
	req := Request{
		Link: c.PostForm("link"),
		Dest: c.PostForm("dest"),
	}
	if !strings.HasPrefix(req.Dest, "http") {
		req.Dest = "https://" + req.Dest
	}
	if req.Link != "" && req.Dest != "" {
		if err := s.DB.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("links"))
			if err := b.Put([]byte(req.Link), []byte(req.Dest)); err != nil {
				return err
			}
			return nil
		}); err != nil {
			log.Fatal(err)
		}
		c.String(http.StatusOK, "Saved | /"+req.Link+" -> "+req.Dest)
	} else {
		c.String(http.StatusInternalServerError, "could not read redirirect link")
	}
}
