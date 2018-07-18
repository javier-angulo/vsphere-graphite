package backend

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/olivere/elastic"
)

// CreateIndexIfNotExists not exists creates Elasticsearch Index if Not Exists
func CreateIndexIfNotExists(e *elastic.Client, index string) error {
	// Use the IndexExists service to check if a specified index exists.
	exists, err := e.IndexExists(index).Do(context.Background())
	if err != nil {
		errlog.Println("Unable to check if Elastic Index exists: ", err)
		return err
	}

	if !exists {
		// Create a new index.
		v := reflect.TypeOf(Point{})

		mapping := MapStr{
			"mappings": MapStr{
				"doc": MapStr{
					"properties": MapStr{},
				},
			},
		}
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			tag := field.Tag.Get("elastic")
			if len(tag) == 0 {
				continue
			}
			tagfields := strings.Split(tag, ",")

			mapping["mappings"].(MapStr)["doc"].(MapStr)["properties"].(MapStr)[field.Name] = MapStr{}

			for _, tagfield := range tagfields {
				tagfieldValues := strings.Split(tagfield, ":")
				mapping["mappings"].(MapStr)["doc"].(MapStr)["properties"].(MapStr)[field.Name].(MapStr)[tagfieldValues[0]] = tagfieldValues[1]
			}
		}
		mappingJSON, err := json.Marshal(mapping)

		if err != nil {
			errlog.Println("Error on Json Marshal")
			return err
		}

		_, err = e.CreateIndex(index).BodyString(string(mappingJSON)).Do(context.Background())

		if err != nil {
			errlog.Println("Error creating Elastic Index:" + index)
			return err
		}
		stdlog.Println("Elastic Index created: " + index)
	}
	return nil
}
