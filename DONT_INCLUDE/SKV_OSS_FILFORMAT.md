Läs mer: https://www.skatteverket.se/foretag/moms/deklareramoms/ansokomattredovisadistansforsaljningionestopshoposs/tekniskbeskrivningonestopshop.4.96cca41179bad4b1aa1d6.html

## Så här gör du

Du behöver skapa en fil i ett ordbehandlingsprogram där uppgifterna kan sparas som oformaterad text. Filen laddar du upp i e-tjänstens sida Deklarera.

### Skapa en fil med deklarationsuppgifterna

Filen med deklarationsuppgifterna ska vara en semikolonavgränsad textfil. Det innebär att du fyller i uppgifterna för varje deklarationsrad på en rad i en särskild ordning. Varje uppgift ska avgränsas med ett semikolon. Du kan fritt välja ett namn på filen då namnet inte har någon betydelse för e-tjänstens funktion.

Så här skriver du innehållet i filen:

Först anges den försäljningen som avser den aktuella perioden enligt följande:

| Rad 1          | Rad 2                                                                                | Rad 3 fram till rättningar                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| :------------- | :----------------------------------------------------------------------------------- | :----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Ange ”OSS_001” | Identifieringsnummer med inledande prefix.                                           | ”SE” (med versaler) för omsättning från Sverige. Vid omsättning från ett medlemsland där du har en fast etablering/lager, ange landskod (med versaler) och registreringsnummer för denna. Om en vara försänds eller transporteras från ett annat medlemsland där du inte har något registreringsnummer för mervärdesskatt eller skatteregistreringsnummer ska en "landsdeklaration" lämnas genom att ange landskod (med versaler) för det land som varan skickats ifrån. |
|                | Deklarationsperiod, kvartal (1,2,3 eller 4) eller månad för importordningen (01-12). | Landskod för konsumtionslandet (MSCON) Använd versaler.                                                                                                                                                                                                                                                                                                                                                                                                                  |
|                | Deklarationsperiod, år (format: ÅÅÅÅ).                                               | Skattesats. Ange med två decimaler och kommatecken som decimalavskiljare.                                                                                                                                                                                                                                                                                                                                                                                                |
|                |                                                                                      | Skattepliktig omsättning, > 0. Ange belopp med två decimaler med kommatecken som decimalavskiljare.                                                                                                                                                                                                                                                                                                                                                                      |
|                |                                                                                      | Skatt att betala, > 0. Ange belopp med två decimaler med kommatecken som decimalavskiljare.                                                                                                                                                                                                                                                                                                                                                                              |
|                |                                                                                      | Om redovisning avser försäljning av vara eller tjänst. Anges med "GOODS" eller "SERVICES"                                                                                                                                                                                                                                                                                                                                                                                |

Sätt ett semikolon mellan uppgifterna på samma rad, se exempel nedan. Avsluta raderna med <CR> (Carriage Return) och <LF> (Line Feed). En symbol syns då vid varje radslut. Skriv tecknen i enlighet med standarder för teckenkodning för västeuropeiska språk (ISO/EIC 8859-1, ”ANSI” eller “Windows”).

Spara filen som en oformaterad textfil. Om du har använt till exempel Microsoft Excel behöver du tänka på att den sparade textfilen inte får innehålla några formler.

Så här kan en korrekt fil för unionsordningen se ut:

OSS_001;◄
SE556000016701;2;2022;◄
SE;DK;25,00;50000,00;12500,00;GOODS;◄
SE;FI;24,00;10060,56;2414,53;GOODS;◄
SE;DE;19,00;20015,00;3802,85;SERVICES;◄
DK12345690;DE;19,00;15090,00;2867,10;SERVICES;◄
DK;DE;19,00;11250,00;2137,50;GOODS;◄
CORRECTIONS◄
2021Q3;FR;500;◄

Så här kan en korrekt fil för importordningen se ut:

OSS_001;◄
IM7520000000;01;2022;◄
SE;DK;25,00;50000,00;12500,00;GOODS;◄
SE;FI;24,00;10060,56;2414,53;GOODS;◄
CORRECTIONS◄
2021M09;FR;500;◄

Du ska bara ange uppgifter för de länder du har haft försäljning till. Ange aldrig beloppet 0 som värde för skattepliktig omsättning, skatt för landet eller justering av skatt att betala.