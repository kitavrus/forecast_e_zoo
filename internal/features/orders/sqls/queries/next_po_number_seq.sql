-- Получает следующее значение sequence для номера PO.
SELECT nextval('orders.po_number_seq')::bigint
