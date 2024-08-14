package utils

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

func PostgresURLToConnectionString(pgurl string) (string, error) {
	u, err := url.Parse(pgurl)
	if err != nil {
		slog.Error("could not connect to postgres", "err", err)
		panic(err)
	}

	username := u.User.Username()
	password, _ := u.User.Password()
	host := u.Host
	dbname := strings.TrimPrefix(u.Path, "/")

	return fmt.Sprintf("user=%v password=%v host=%v dbname=%v sslmode=require", username, password, host, dbname), nil
}

type PostgresNotificationListener struct {
	postgresURI string
	listener    *pq.Listener
	minReconn   time.Duration
	maxReconn   time.Duration
	mbNotifs    Mailbox[string]
	chStop      chan struct{}
	wgDone      sync.WaitGroup
	closeOnce   sync.Once
}

func NewPostgresNotificationListener(postgresURI string, minReconn time.Duration, maxReconn time.Duration) *PostgresNotificationListener {
	return &PostgresNotificationListener{
		mbNotifs:    NewMailbox[string](1000),
		postgresURI: postgresURI,
		minReconn:   minReconn,
		maxReconn:   maxReconn,
		chStop:      make(chan struct{}),
	}
}

func (l *PostgresNotificationListener) Listen(channel string) error {
	l.listener = pq.NewListener(l.postgresURI, l.minReconn, l.maxReconn, l.handleMetaEvent)
	err := l.listener.Listen(channel)
	if err != nil {
		slog.Error("failed to listen to postgres channel", "channel", channel, "err", err)
		return err
	}

	pingTicker := time.NewTicker(15 * time.Second)

	l.wgDone.Add(1)
	go func() {
		defer l.wgDone.Done()
		defer pingTicker.Stop()

	Outer:
		for {
			select {
			case <-l.chStop:
				return

			case <-pingTicker.C:
				err := l.listener.Ping()
				if err != nil {
					slog.Error("postgres listener ping failed", "channel", channel)
					continue Outer
				}

			case notif, open := <-l.listener.NotificationChannel():
				if !open {
					slog.Warn("postgres listener channel closed", "channel", channel)
					return
				}
				l.mbNotifs.Deliver(notif.Extra)
			}
		}
	}()
	return nil
}

func (l *PostgresNotificationListener) Close() error {
	var err error
	l.closeOnce.Do(func() {
		close(l.chStop)
		err = l.listener.Close()
		l.wgDone.Wait()
	})
	return err
}

func (l *PostgresNotificationListener) handleMetaEvent(ev pq.ListenerEventType, err error) {
	if err != nil {
		slog.Error("error in postgres listener", "error", err)
	}
}

func (l *PostgresNotificationListener) RetrieveAll() []string {
	return l.mbNotifs.RetrieveAll()
}

func (l *PostgresNotificationListener) Notify() <-chan struct{} {
	return l.mbNotifs.Notify()
}

type PostgresQueue[T any] struct {
	postgresURI         string
	db                  *sqlx.DB
	listener            *PostgresNotificationListener
	notificationChannel string
	notificationQuery   string
	catchupQuery        string

	mbQueue   Mailbox[T]
	chStop    chan struct{}
	wgDone    sync.WaitGroup
	closeOnce sync.Once
}

func NewPostgresQueue[T any](postgresURI string, db *sqlx.DB, notificationChannel string, notificationQuery, catchupQuery string) *PostgresQueue[T] {
	return &PostgresQueue[T]{
		postgresURI:         postgresURI,
		db:                  db,
		listener:            NewPostgresNotificationListener(postgresURI, 1*time.Second, 10*time.Second),
		notificationChannel: notificationChannel,
		notificationQuery:   notificationQuery,
		catchupQuery:        catchupQuery,
		chStop:              make(chan struct{}),
		mbQueue:             NewMailbox[T](1000),
	}
}

func (q *PostgresQueue[T]) Start() error {
	err := q.listener.Listen(q.notificationChannel)
	if err != nil {
		return err
	}

	fetchTicker := time.NewTicker(15 * time.Second)

	q.wgDone.Add(1)
	go func() {
		defer q.wgDone.Done()
		defer fetchTicker.Stop()

	Outer:
		for {
			select {
			case <-q.chStop:
				return

			case <-fetchTicker.C:
				// Occasionally run the catchup query in case we missed notifications
				var rows []T
				err := q.db.SelectContext(context.TODO(), &rows, q.catchupQuery)
				if err != nil {
					slog.Error("failed to catchup", "err", err)
					continue Outer
				}
				for _, row := range rows {
					q.mbQueue.Deliver(row)
				}

			case <-q.listener.Notify():
				// When we get notifications, run the notification query
				notifs := q.listener.RetrieveAll()

				var rows []T
				err := q.db.SelectContext(context.TODO(), &rows, q.notificationQuery, pq.Array(notifs))
				if err != nil {
					slog.Error("failed to fetch notifications", "err", err)
					continue Outer
				}

				for _, row := range rows {
					q.mbQueue.Deliver(row)
				}
			}
		}
	}()
	return nil
}

func (q *PostgresQueue[T]) Notify() <-chan struct{} {
	return q.mbQueue.Notify()
}

func (q *PostgresQueue[T]) RetrieveAll() []T {
	return q.mbQueue.RetrieveAll()
}

func (q *PostgresQueue[T]) Close() error {
	var err error
	q.closeOnce.Do(func() {
		close(q.chStop)
		err = q.listener.Close()
		q.wgDone.Wait()
	})
	return err
}
