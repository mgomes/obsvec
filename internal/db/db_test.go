package db

import (
	"os"
	"path/filepath"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
)

func setupTestDB(t *testing.T) (*DB, func()) {
	tmpDir, err := os.MkdirTemp("", "obsvec-db-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath, 4) // Small dimension for testing
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestDatabaseOpen(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	if db == nil {
		t.Fatal("expected non-nil database")
	}
}

func TestDocumentCRUD(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Insert
	docID, err := db.UpsertDocument("test/path.md", "Test Title", 1000, 2000)
	if err != nil {
		t.Fatalf("failed to insert document: %v", err)
	}

	if docID == 0 {
		t.Error("expected non-zero document ID")
	}

	// Read
	doc, err := db.GetDocument("test/path.md")
	if err != nil {
		t.Fatalf("failed to get document: %v", err)
	}

	if doc == nil {
		t.Fatal("expected document, got nil")
	}

	if doc.Title != "Test Title" {
		t.Errorf("expected title 'Test Title', got '%s'", doc.Title)
	}

	if doc.ModifiedAt != 1000 {
		t.Errorf("expected modified_at 1000, got %d", doc.ModifiedAt)
	}

	// Update (upsert)
	_, err = db.UpsertDocument("test/path.md", "Updated Title", 1500, 2500)
	if err != nil {
		t.Fatalf("failed to update document: %v", err)
	}

	doc, _ = db.GetDocument("test/path.md")
	if doc.Title != "Updated Title" {
		t.Errorf("expected updated title, got '%s'", doc.Title)
	}

	// Delete
	err = db.DeleteDocument("test/path.md")
	if err != nil {
		t.Fatalf("failed to delete document: %v", err)
	}

	doc, _ = db.GetDocument("test/path.md")
	if doc != nil {
		t.Error("expected nil after delete")
	}
}

func TestChunkOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a document first
	docID, _ := db.UpsertDocument("test.md", "Test", 1000, 2000)

	// Insert chunk
	chunkID, err := db.InsertChunk(docID, "This is chunk content", 1, 10, "Heading")
	if err != nil {
		t.Fatalf("failed to insert chunk: %v", err)
	}

	if chunkID == 0 {
		t.Error("expected non-zero chunk ID")
	}

	// Get chunk
	chunk, err := db.GetChunk(chunkID)
	if err != nil {
		t.Fatalf("failed to get chunk: %v", err)
	}

	if chunk.Content != "This is chunk content" {
		t.Errorf("expected content 'This is chunk content', got '%s'", chunk.Content)
	}

	if chunk.Heading != "Heading" {
		t.Errorf("expected heading 'Heading', got '%s'", chunk.Heading)
	}

	// Delete chunks for document
	err = db.DeleteChunksForDocument(docID)
	if err != nil {
		t.Fatalf("failed to delete chunks: %v", err)
	}

	chunk, _ = db.GetChunk(chunkID)
	if chunk != nil {
		t.Error("expected chunk to be deleted")
	}
}

func TestEmbeddingOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	docID, _ := db.UpsertDocument("test.md", "Test", 1000, 2000)
	chunkID, _ := db.InsertChunk(docID, "Content", 1, 5, "")

	// Insert embedding (4 dimensions as configured)
	embedding := []float32{0.1, 0.2, 0.3, 0.4}
	embBytes, err := sqlite_vec.SerializeFloat32(embedding)
	if err != nil {
		t.Fatalf("failed to serialize embedding: %v", err)
	}

	err = db.InsertEmbedding(chunkID, embBytes)
	if err != nil {
		t.Fatalf("failed to insert embedding: %v", err)
	}

	// Search similar
	queryEmb := []float32{0.1, 0.2, 0.3, 0.4}
	queryBytes, _ := sqlite_vec.SerializeFloat32(queryEmb)

	results, err := db.SearchSimilar(queryBytes, 10)
	if err != nil {
		t.Fatalf("failed to search: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].Content != "Content" {
		t.Errorf("expected content 'Content', got '%s'", results[0].Content)
	}
}

func TestDocumentCount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	count, _ := db.DocumentCount()
	if count != 0 {
		t.Errorf("expected 0 documents, got %d", count)
	}

	_, _ = db.UpsertDocument("a.md", "A", 1000, 2000)
	_, _ = db.UpsertDocument("b.md", "B", 1000, 2000)

	count, _ = db.DocumentCount()
	if count != 2 {
		t.Errorf("expected 2 documents, got %d", count)
	}
}

func TestChunkCount(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	docID, _ := db.UpsertDocument("test.md", "Test", 1000, 2000)

	count, _ := db.ChunkCount()
	if count != 0 {
		t.Errorf("expected 0 chunks, got %d", count)
	}

	_, _ = db.InsertChunk(docID, "Chunk 1", 1, 5, "")
	_, _ = db.InsertChunk(docID, "Chunk 2", 6, 10, "")

	count, _ = db.ChunkCount()
	if count != 2 {
		t.Errorf("expected 2 chunks, got %d", count)
	}
}

func TestGetAllDocuments(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	_, _ = db.UpsertDocument("a.md", "A", 1000, 2000)
	_, _ = db.UpsertDocument("b.md", "B", 1000, 2000)
	_, _ = db.UpsertDocument("c.md", "C", 1000, 2000)

	docs, err := db.GetAllDocuments()
	if err != nil {
		t.Fatalf("failed to get all documents: %v", err)
	}

	if len(docs) != 3 {
		t.Errorf("expected 3 documents, got %d", len(docs))
	}
}
