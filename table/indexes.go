package table

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureTableIndexes creates the necessary compound and single-field B-tree indexes
// for high-performance keyset/cursor pagination, sorting, search, and tenant isolation.
// Calling this on service startup guarantees that table queries run via index seeks rather than COLLSCAN.
func EnsureTableIndexes(
	ctx context.Context,
	col *mongo.Collection,
	tenantScoped bool,
	searchFields []string,
	sortFields []string,
) error {
	if col == nil {
		return fmt.Errorf("collection is nil")
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var models []mongo.IndexModel

	if tenantScoped {
		// 1. Compound index for default sort & tenant isolation: {tenant_id: 1, deleted_at: 1, created_at: -1}
		models = append(models, mongo.IndexModel{
			Keys: bson.D{
				{Key: "tenant_id", Value: 1},
				{Key: "deleted_at", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_table_tenant_deleted_created"),
		})

		// 2. Compound indexes for allowed sort fields
		for _, sf := range sortFields {
			sf = strings.TrimSpace(sf)
			if sf == "" || sf == "created_at" || sf == "_id" {
				continue
			}
			idxName := fmt.Sprintf("idx_table_tenant_deleted_%s", sf)
			models = append(models, mongo.IndexModel{
				Keys: bson.D{
					{Key: "tenant_id", Value: 1},
					{Key: "deleted_at", Value: 1},
					{Key: sf, Value: 1},
				},
				Options: options.Index().SetName(idxName),
			})
		}

		// 3. Search field prefix indexes
		for _, s := range searchFields {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			idxName := fmt.Sprintf("idx_table_tenant_search_%s", s)
			models = append(models, mongo.IndexModel{
				Keys: bson.D{
					{Key: "tenant_id", Value: 1},
					{Key: s, Value: 1},
				},
				Options: options.Index().SetName(idxName),
			})
		}
	} else {
		// Non-tenant scoped collection
		models = append(models, mongo.IndexModel{
			Keys: bson.D{
				{Key: "deleted_at", Value: 1},
				{Key: "created_at", Value: -1},
			},
			Options: options.Index().SetName("idx_table_deleted_created"),
		})

		for _, sf := range sortFields {
			sf = strings.TrimSpace(sf)
			if sf == "" || sf == "created_at" || sf == "_id" {
				continue
			}
			idxName := fmt.Sprintf("idx_table_sort_%s", sf)
			models = append(models, mongo.IndexModel{
				Keys: bson.D{
					{Key: sf, Value: 1},
				},
				Options: options.Index().SetName(idxName),
			})
		}

		for _, s := range searchFields {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			idxName := fmt.Sprintf("idx_table_search_%s", s)
			models = append(models, mongo.IndexModel{
				Keys: bson.D{
					{Key: s, Value: 1},
				},
				Options: options.Index().SetName(idxName),
			})
		}
	}

	_, err := col.Indexes().CreateMany(timeoutCtx, models)
	if err != nil {
		log.Printf("[Warning] Failed to ensure indexes for collection %s: %v", col.Name(), err)
		return err
	}

	return nil
}

// EnsureIndex creates a single index with optional uniqueness and custom name.
func EnsureIndex(ctx context.Context, col *mongo.Collection, keys bson.D, unique bool, name string) error {
	if col == nil {
		return fmt.Errorf("collection is nil")
	}

	opts := options.Index()
	if unique {
		opts.SetUnique(true)
	}
	if name != "" {
		opts.SetName(name)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err := col.Indexes().CreateOne(timeoutCtx, mongo.IndexModel{
		Keys:    keys,
		Options: opts,
	})
	return err
}
