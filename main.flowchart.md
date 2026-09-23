graph TD
  node_login_ui@{label: "Login screens<br>[page.tsx]", shape: rect, x-posX: 594, x-posY: 122}
  node_admin_ui@{label: "Admin screens<br>[layout.tsx]", shape: rect, x-posX: 802, x-posY: 122}
  node_api_client@{label: "API client<br>[apiClient.ts]", shape: rect, x-posX: 802, x-posY: 220}
  node_api_server@{label: "API server<br>[main.go]", shape: rect, x-posX: 802, x-posY: 318}
  node_http_routes@{label: "HTTP routes<br>[routes.go]", shape: rect, x-posX: 500, x-posY: 416}
  node_request_middleware@{label: "Request middleware<br>[auth_middleware.go]", shape: rect, x-posX: 500, x-posY: 514}
  node_auth_handlers@{label: "Auth handlers<br>[auth_handler.go]", shape: rect, x-posX: 64, x-posY: 612}
  node_crm_handlers@{label: "CRM handlers", shape: rect, x-posX: 292, x-posY: 612}
  node_sales_handlers@{label: "Sales handlers", shape: rect, x-posX: 500, x-posY: 612}
  node_dashboard_handlers@{label: "Dashboard handlers", shape: rect, x-posX: 708, x-posY: 612}
  node_auth_service@{label: "Auth service<br>[auth_service.go]", shape: rect, x-posX: 64, x-posY: 710}
  node_domain_services@{label: "Domain services", shape: rect, x-posX: 708, x-posY: 710}
  node_token_services@{label: "Token services", shape: rect, x-posX: 272, x-posY: 710}
  node_email_service@{label: "Email service<br>[email_service.go]", shape: rect, x-posX: 802, x-posY: 416}
  node_repositories@{label: "Repositories", shape: rect, x-posX: 708, x-posY: 808}
  node_db_connection@{label: "DB connection<br>[db.go]", shape: rect, x-posX: 916, x-posY: 808}
  node_mysql@{label: "MySQL database", shape: rect, x-posX: 956, x-posY: 906}
  node_crm_importers@{label: "CRM importers<br>[main.go]", shape: rect, x-posX: 1124, x-posY: 808}
  node_erp_importers@{label: "ERP importers<br>[main.go]", shape: rect, x-posX: 1332, x-posY: 808}
  node_web_user@{label: "Web user", shape: rect, x-posX: 594, x-posY: 24}
  node_admin_user@{label: "Administrator", shape: rect, x-posX: 802, x-posY: 24}
  node_csv_sources@{label: "ERP CRM CSVs", shape: rect, x-posX: 1332, x-posY: 906}
  node_smtp@{label: "SMTP server", shape: rect, x-posX: 802, x-posY: 514}
  node_web_user -- opens login --> node_login_ui;
  node_admin_user -- uses console --> node_admin_ui;
  node_login_ui -- submits credentials --> node_api_client;
  node_admin_ui -- calls API --> node_api_client;
  node_api_client -- sends requests --> node_api_server;
  node_api_server -- serves routes --> node_http_routes;
  node_http_routes -- wraps handlers --> node_request_middleware;
  node_request_middleware -- dispatches auth --> node_auth_handlers;
  node_request_middleware -- dispatches CRM --> node_crm_handlers;
  node_request_middleware -- dispatches sales --> node_sales_handlers;
  node_request_middleware -- dispatches metrics --> node_dashboard_handlers;
  node_auth_handlers -- authenticates users --> node_auth_service;
  node_auth_handlers -- manages refresh --> node_token_services;
  node_auth_handlers -- updates users --> node_repositories;
  node_crm_handlers -- runs CRM logic --> node_domain_services;
  node_sales_handlers -- runs sales logic --> node_domain_services;
  node_dashboard_handlers -- loads metrics --> node_domain_services;
  node_auth_service -- reads users --> node_repositories;
  node_token_services -- stores tokens --> node_repositories;
  node_domain_services -- persists domain --> node_repositories;
  node_api_server -- opens database --> node_db_connection;
  node_repositories -- reads writes --> node_mysql;
  node_db_connection -- connects to --> node_mysql;
  node_api_server -- configures email --> node_email_service;
  node_email_service -- sends mail --> node_smtp;
  node_crm_importers -- reads CRM CSVs --> node_csv_sources;
  node_crm_importers -- upserts CRM --> node_mysql;
  node_erp_importers -- reads ERP CSVs --> node_csv_sources;
  node_erp_importers -- upserts ERP --> node_mysql;
