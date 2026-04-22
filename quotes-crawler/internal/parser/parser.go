package parser

import "quote-crawler/internal/models"

type Parser interface {
	Parse(html string) ([]models.Quote, error)
}
