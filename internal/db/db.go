package db

import (
	"database/sql"
	"fmt"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn     *sql.DB
	embedDim int
}

type Document struct {
	ID         int64
	Path       string
	Title      string
	ModifiedAt int64
	IndexedAt  int64
}

type Chunk struct {
	ID        int64
	DocID     int64
	Content   string
	StartLine int
	EndLine   int
	Heading   string
}

type ChunkWithScore struct {
	Chunk
	Distance float64
	Path     string
}

func init() {
	sqlite_vec.Auto()
}

func Open(path string, embedDim int) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{conn: conn, embedDim: embedDim}
	if err := db.init(); err != nil {
		conn.Close() //nolint:errcheck
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) init() error {
	var vecVersion string
	if err := db.conn.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
		return fmt.Errorf("sqlite-vec not available: %w", err)
	}

	schema := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY,
			path TEXT UNIQUE NOT NULL,
			title TEXT,
			modified_at INTEGER,
			indexed_at INTEGER
		);

		CREATE TABLE IF NOT EXISTS chunks (
			id INTEGER PRIMARY KEY,
			doc_id INTEGER REFERENCES documents(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			start_line INTEGER,
			end_line INTEGER,
			heading TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(doc_id);
		CREATE INDEX IF NOT EXISTS idx_documents_path ON documents(path);

		CREATE VIRTUAL TABLE IF NOT EXISTS vec_chunks USING vec0(
			chunk_id INTEGER PRIMARY KEY,
			embedding float[%d]
		);
	`, db.embedDim)

	_, err := db.conn.Exec(schema)
	return err
}

func (db *DB) GetDocument(path string) (*Document, error) {
	var doc Document
	err := db.conn.QueryRow(
		"SELECT id, path, title, modified_at, indexed_at FROM documents WHERE path = ?",
		path,
	).Scan(&doc.ID, &doc.Path, &doc.Title, &doc.ModifiedAt, &doc.IndexedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (db *DB) UpsertDocument(path, title string, modifiedAt, indexedAt int64) (int64, error) {
	result, err := db.conn.Exec(`
		INSERT INTO documents (path, title, modified_at, indexed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			modified_at = excluded.modified_at,
			indexed_at = excluded.indexed_at
	`, path, title, modifiedAt, indexedAt)
	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		var docID int64
		err = db.conn.QueryRow("SELECT id FROM documents WHERE path = ?", path).Scan(&docID)
		if err != nil {
			return 0, err
		}
		return docID, nil
	}
	return id, nil
}

func (db *DB) DeleteDocument(path string) error {
	var docID int64
	err := db.conn.QueryRow("SELECT id FROM documents WHERE path = ?", path).Scan(&docID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	rows, err := db.conn.Query("SELECT id FROM chunks WHERE doc_id = ?", docID)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck

	var chunkIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		chunkIDs = append(chunkIDs, id)
	}

	for _, chunkID := range chunkIDs {
		if _, err := db.conn.Exec("DELETE FROM vec_chunks WHERE chunk_id = ?", chunkID); err != nil {
			return err
		}
	}

	if _, err := db.conn.Exec("DELETE FROM chunks WHERE doc_id = ?", docID); err != nil {
		return err
	}

	_, err = db.conn.Exec("DELETE FROM documents WHERE id = ?", docID)
	return err
}

func (db *DB) DeleteChunksForDocument(docID int64) error {
	rows, err := db.conn.Query("SELECT id FROM chunks WHERE doc_id = ?", docID)
	if err != nil {
		return err
	}
	defer rows.Close() //nolint:errcheck

	var chunkIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		chunkIDs = append(chunkIDs, id)
	}

	for _, chunkID := range chunkIDs {
		if _, err := db.conn.Exec("DELETE FROM vec_chunks WHERE chunk_id = ?", chunkID); err != nil {
			return err
		}
	}

	_, err = db.conn.Exec("DELETE FROM chunks WHERE doc_id = ?", docID)
	return err
}

func (db *DB) InsertChunk(docID int64, content string, startLine, endLine int, heading string) (int64, error) {
	result, err := db.conn.Exec(`
		INSERT INTO chunks (doc_id, content, start_line, end_line, heading)
		VALUES (?, ?, ?, ?, ?)
	`, docID, content, startLine, endLine, heading)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (db *DB) InsertEmbedding(chunkID int64, embedding []byte) error {
	_, err := db.conn.Exec(
		"INSERT INTO vec_chunks (chunk_id, embedding) VALUES (?, ?)",
		chunkID, embedding,
	)
	return err
}

func (db *DB) SearchSimilar(queryEmbedding []byte, limit int) ([]ChunkWithScore, error) {
	rows, err := db.conn.Query(`
		SELECT
			v.chunk_id,
			v.distance,
			c.doc_id,
			c.content,
			c.start_line,
			c.end_line,
			c.heading,
			d.path
		FROM vec_chunks v
		JOIN chunks c ON c.id = v.chunk_id
		JOIN documents d ON d.id = c.doc_id
		WHERE v.embedding MATCH ? AND k = ?
		ORDER BY v.distance
	`, queryEmbedding, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var results []ChunkWithScore
	for rows.Next() {
		var chunk ChunkWithScore
		err := rows.Scan(
			&chunk.ID,
			&chunk.Distance,
			&chunk.DocID,
			&chunk.Content,
			&chunk.StartLine,
			&chunk.EndLine,
			&chunk.Heading,
			&chunk.Path,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, chunk)
	}

	return results, rows.Err()
}

func (db *DB) GetAllDocuments() ([]Document, error) {
	rows, err := db.conn.Query("SELECT id, path, title, modified_at, indexed_at FROM documents")
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	var docs []Document
	for rows.Next() {
		var doc Document
		if err := rows.Scan(&doc.ID, &doc.Path, &doc.Title, &doc.ModifiedAt, &doc.IndexedAt); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

func (db *DB) GetChunk(id int64) (*Chunk, error) {
	var chunk Chunk
	err := db.conn.QueryRow(
		"SELECT id, doc_id, content, start_line, end_line, heading FROM chunks WHERE id = ?",
		id,
	).Scan(&chunk.ID, &chunk.DocID, &chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Heading)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &chunk, nil
}

func (db *DB) GetChunksForRerank(chunkIDs []int64) ([]Chunk, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	query := "SELECT id, doc_id, content, start_line, end_line, heading FROM chunks WHERE id IN ("
	args := make([]interface{}, len(chunkIDs))
	for i, id := range chunkIDs {
		if i > 0 {
			query += ", "
		}
		query += "?"
		args[i] = id
	}
	query += ")"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	chunkMap := make(map[int64]Chunk)
	for rows.Next() {
		var chunk Chunk
		if err := rows.Scan(&chunk.ID, &chunk.DocID, &chunk.Content, &chunk.StartLine, &chunk.EndLine, &chunk.Heading); err != nil {
			return nil, err
		}
		chunkMap[chunk.ID] = chunk
	}

	result := make([]Chunk, 0, len(chunkIDs))
	for _, id := range chunkIDs {
		if chunk, ok := chunkMap[id]; ok {
			result = append(result, chunk)
		}
	}

	return result, rows.Err()
}

func (db *DB) DocumentCount() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM documents").Scan(&count)
	return count, err
}

func (db *DB) ChunkCount() (int, error) {
	var count int
	err := db.conn.QueryRow("SELECT COUNT(*) FROM chunks").Scan(&count)
	return count, err
}
