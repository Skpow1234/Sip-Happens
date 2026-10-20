-- DIAN electronic-invoice fields (migration 030 per pkg/domain/invoice.go).
alter table invoices
  add column cufe text,
  add column resolution_number text,
  add column resolution_prefix text,
  add column sequence_number bigint,
  add column xml_url text,
  add column pdf_url text,
  add column issued_at timestamptz;
