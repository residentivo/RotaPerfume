0-Instale as configurações nessa pasta para melhorar o desempenho da execução do Claude
1-Crium gitIgnore para essa pasta que não enviem os arquivos .env
2-Use as chaves de acesso que estaos no arquivo .env
3-Tenho um banco de dados rodando no endereço mysql://localhost:3306/ usando o acesso golang e preciso que vc pegue todos os csvs da pasta dados e crie os scripts para criação das tabelas nesse banco. crie todas as ligações entre as tabelas inferindo as colunas que fazem as referencias entres os IDs. 
4-Criar também uma tabela de usuarios associadas aos vendedores. Essa tabela tem que conter se o login é um vendor ou um gerente ou do Rh. Deve conter uma coluna para dizer quem é o login que genencia o login
5-Apos criar os scripts executar no banco rotaperfumes esses scripts e que em seguida, suba os dados dos CSVS nessas tabelas. 
6-As apis criadas nos proximos passos devem conter um sistema de acesso com token, onde ele valida usuario e senha para gerar uma token, a linguagem será go .
7-O proximo passo é fazer uma api para gerenciar os Logins e Vendedores chamado RH.
8-O proximo pasos é uma API com as regras de negocio de um CRM que usem as tabelas que forem da pasta Dados\CRM. 
8-O proximo pasos é uma API com as regras de negocio de um ERP que usem as tabelas que forem da pasta Dados\ERP.
9-Para as paginas será usadno nodejs para fazer o backend e bootstrap padrão, as tokens das apis serão geradas por esse bakcend e gravado no cookie para as proximas reguisições. Os carregametos das telas devem ser feitas por endpoins para o backend que fara uma nova reguisição para API. Todas devem ter uma tela de login para acessar o sistema. 
10-Deve haver uma sistema de telas para o RH gerencias os daods de login e vendedores que irão se cominunicar com os dados que virão da api de RH.
11-Em outro sistema de telas deve conter todas as regras de negocio de um CRM para a telas que irão se cominunicar com os dados que virão da api de CRM.
11-Em outro sistema de telas deve conter todas as regras de negocio de um ERP para a telas que irão se cominunicar com os dados que virão da api de ERP.
