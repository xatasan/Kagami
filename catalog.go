package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"os"
	"time"
)

func mkCatalog(name, board string) error {
	var (
		page = "./catalog-%d.html"
		opc  int // OP count
		sort string
	)

	if sticky {
		sort = "posts.sticky DESC, posts.time DESC"
	} else {
		sort = "posts.time DESC"
	}

	db.QueryRow(`SELECT count(1) FROM posts WHERE op;`).Scan(&opc)
	pages := int(math.Ceil(float64(opc) / float64(tpp)))
	q, err := db.Prepare(`
        SELECT posts.postno, posts.time, posts.name,
               posts.tripcode, posts.capcode,
               posts.subject, posts.comment, posts.country,
               (SELECT files.tfilename
                FROM files
                WHERE (files.id = (SELECT links.file
                                   FROM links
                                   WHERE links.post = posts.postno
                                   LIMIT 1)))
        FROM posts
        WHERE posts.op
        ORDER BY ` + sort + `
        LIMIT ? OFFSET ?`)
	if err != nil {
		return err
	}

	debugL("Creating %d catalog page(s) for %d threads", pages, opc)
	for P := 1; P <= pages; P++ {
		file := fmt.Sprintf(page, P)
		_, err := os.Stat(file)
		if !os.IsNotExist(err) && P < pages {
			debugL("Doesn't have to create catalog page %d", P)
		}

		rows, err := q.Query( // q,
			tpp, (P-1)*tpp)
		if err != nil {
			log.Fatal(err)
		}

		posts := make(chan Post)
		go func() {
			for rows.Next() {
				var (
					com string
					tmb string
					p   Post
				)
				rows.Scan(
					&p.PostNumber,
					&p.Time,
					&p.Name,
					&p.Tripcode,
					&p.Capcode,
					&p.Subject,
					&com,
					&p.Country,
					&tmb,
					// &p.Images
				)
				p.Comment = template.HTML(com)
				p.Files = []File{{Thumbnail: tmb}}
				posts <- p
			}
			close(posts)
		}()

		f, err := os.Create(file)
		if err != nil {
			log.Fatal(err)
		}

		pr, pw := io.Pipe()
		defer pr.Close()
		go func() {
			defer pw.Close()
			err = t.Lookup("catalog.tmpl").Execute(pw, struct {
				T          <-chan Post // posts
				P, A       int         // page, count (amount)
				Next, Prev int         // page numbers
				N, B       string      // name and board
				U          time.Time
			}{
				posts,
				P, pages,
				P + 1, P - 1,
				name, board,
				time.Now()})
			if err != nil {
				log.Fatal(err)
			}
		}()

		m.Minify("text/html", f, pr)
		f.Close()
		verboseL("Wrote catalog page %d", P)
	}
	if err := os.Link("catalog-1.html", "catalog.html"); err != nil && !os.IsExist(err) {
		log.Fatalln(err)
	}
	return nil
}
