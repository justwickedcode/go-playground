package parser

import "quotes-crawler/internal/models"

type Parser interface {
	Parse(html string) ([]models.Quote, error)
}
