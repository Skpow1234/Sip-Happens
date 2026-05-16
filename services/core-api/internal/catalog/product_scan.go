package catalog

import (
	"database/sql"

	"sip-happens/pkg/domain"
)

const productSelectCols = `p.id, p.sku, p.name, p.brewery, p.style, p.abv, p.volume_ml, p.invima_reg, p.image_url, p.is_active, p.created_at, p.updated_at`

func scanProduct(scanner interface {
	Scan(dest ...any) error
}) (domain.Product, error) {
	var p domain.Product
	var imageURL sql.NullString
	err := scanner.Scan(
		&p.ID, &p.SKU, &p.Name, &p.Brewery, &p.Style, &p.ABV, &p.VolumeML, &p.InvimaReg, &imageURL, &p.IsActive, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return domain.Product{}, err
	}
	if imageURL.Valid {
		p.ImageURL = imageURL.String
	}
	return p, nil
}
