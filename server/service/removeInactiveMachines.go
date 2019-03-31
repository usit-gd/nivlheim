package main

import (
	"database/sql"
	"log"
	"time"

	"github.com/usit-gd/nivlheim/server/service/utility"
)

type removeInactiveMachinesJob struct{}

func init() {
	RegisterJob(removeInactiveMachinesJob{})
}

func (job removeInactiveMachinesJob) HowOften() time.Duration {
	return time.Hour * 24
}

func (job removeInactiveMachinesJob) Run(db *sql.DB) {
	// Archive machines (delete the hostinfo entry, but keep the files)
	rows, err := db.Query("SELECT certfp,extract(day from now()-lastseen) FROM hostinfo")
	if err != nil {
		log.Print(err)
		return
	}
	defer rows.Close()
	var acount, dcount int
	for rows.Next() {
		var certfp string
		var days64 sql.NullInt64
		err = rows.Scan(&certfp, &days64)
		if err != nil {
			log.Print(err)
			break
		}
		days := int(days64.Int64)
		if days >= archiveDayLimit {
			err = utility.RunStatementsInTransaction(db, []string{
				"UPDATE files SET current=false WHERE certfp=$1",
				"DELETE FROM hostinfo WHERE certfp=$1",
			}, certfp)
			if err != nil {
				log.Print(err)
			} else {
				acount++
			}
		}
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
	}
	rows.Close()
	
	// Delete old files where I haven't heard from the machine in a long time
	rows, err = db.Query("SELECT DISTINCT certfp FROM files GROUP BY certfp"+
		" HAVING max(received) < now() - interval '$1 days'", deleteDayLimit)
	if err != nil {
		log.Print(err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var certfp string
		err = rows.Scan(&certfp)
		if err != nil {
			log.Print(err)
			break
		}
		err = utility.RunStatementsInTransaction(db, []string{
			"DELETE FROM hostinfo WHERE certfp=$1",
			"DELETE FROM files WHERE certfp=$1",
		}, certfp)
		if err != nil {
			log.Print(err)
		} else {
			dcount++
		}
	}
	if err = rows.Err(); err != nil {
		log.Print(err)
	}
	if acount > 0 || dcount > 0 {
		log.Printf("Archived %d machines, deleted %d machines", acount, dcount)
	}
}
