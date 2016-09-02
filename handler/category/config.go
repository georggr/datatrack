package category

import (
	"github.com/pylls/datatrack/config"
	"github.com/pylls/datatrack/handler"
)

const baseURL = config.APIURL + "/type"
const withCategory = "/:categoryId"

// Handlers contain handlers.
var Handlers = handler.Handlers{
	handler.Handler{
		Name:        "attribute type categories",
		Method:      "get",
		Url:         baseURL + withCategory,
		Description: "retrieve all types belonging to a (sub)category",
		Handle:      categoryHandler}}
