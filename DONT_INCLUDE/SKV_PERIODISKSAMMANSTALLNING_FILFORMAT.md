Läs mer: https://www.skatteverket.se/foretag/moms/deklareramoms/periodisksammanstallningforvarorochtjanster/lamnaperiodisksammanstallningmedfiloverforing.4.7eada0316ed67d72822104.html

"Lämna in senast den 25:e i månaden efter den period försäljningen skett (månad eller kvartal)."

## Filformat och filens innehåll

Filen som du för över via filöverföringstjänsten ska vara en textfil som är avgränsad med semikolon. En sådan fil består av ett antal rader där uppgifterna skiljs åt av ett semikolon. Filen ska enbart innehålla text och kan ha ändelsen .txt eller .csv. Det är möjligt att exportera innehållet i en Excel-fil till .csv-format. En fil med uppgifter för periodisk sammanställning ska innehålla nedanstående uppgifter. Du hittar exempel på hur en fil kan se ut längst ner på sidan.

### Vad filen innehåller

På första raden anger du SKV574008 för att visa att filen avser en periodisk sammanställning.

### Uppgiftsskyldig, vilken månad eller kvartal som avses med mera

På den andra raden anger du i tur och ordning:

- Det 12-siffriga momsregistreringsnumret för den som är skyldig att lämna uppgifterna. Med eller utan landskoden SE.
- Månads- eller kvartalskoden för den månad eller det kalenderkvartal uppgifterna gäller, till exempel 2212 för december 2022, 2301 för januari 2023, 22-4 för fjärde kvartalet 2022 eller 23-1 för första kvartalet 2023.
- Namnet på personen som är ansvarig för de lämnade uppgifterna (högst 35 tecken).
- Telefonnummer till den som är ansvarig för uppgifterna (endast siffror, med bindestreck efter riktnumret eller utlandsnummer, som inleds med plustecken [högst 17 tecken]).
- Frivillig uppgift om e-postadress till den som är ansvarig för uppgifterna.

### Uppgifter om försäljningar och överföringar

På den tredje och följande rader anges i tur och ordning:

- Köparens momsregistreringsnummer (VAT-nummer) med inledande landskod.
- Värde av varuleveranser (högst 12 siffror inklusive eventuellt minustecken).
- Värde av trepartshandel (högst 12 siffror inklusive eventuellt minustecken).
- Värde av tjänster enligt huvudregeln (högst 12 siffror inklusive eventuellt minustecken).

Om du inte har haft någon försäljning ska du markera med ett semikolon i kolumnen. I det fall du ska redovisa ett negativt belopp, vilket exempelvis blir aktuellt vid redovisning av krediteringar, anger du ett minustecken framför beloppet. Om det sammanlagda beloppet blir 0 kronor anger du 0. Detta gör du även om du ska ta bort en tidigare redovisad försäljning. Om du använder förenklingsreglerna vid överföring till avropslager till ett annat EU-land redovisar du överföringen utan belopp. Du kan i vissa fall byta till en annan redovisningsperiod.

## Exempel på filer

De uppgifter som visas nedan utgör exempel för att visa hur en fil ska se ut. Innehållet i filen du skapar, momsregistreringsnummer, period, VAT-nummer och liknande, måste anpassas utifrån den redovisning du ska lämna.

### Exempel 1

Fil med uppgifter om varuförsäljning, trepartshandel, tjänsteförsäljning samt överföring till avropslager. Bokstaven X i sista raden indikerar att uppgifterna avser en överföring till den avsedda köparen med VAT-nummer DE262875793.

```xml
SKV574008;
556000016701;2001;Per Persson;0123-45690; post@filmkopia.se
FI01409351;16400;;;
DK31283522;21700;13600;;
ESA28480514;3200;;15300;
DE262875793;X;;
```

### Exempel 2

Fil med ändring av tidigare redovisade uppgifter avseende överföring till avropslager. Uppgifterna visa att varor har återförts till Sverige från lagret i Tyskland. I filen ska anges den tidigare avsedda köparen samt bokstaven Y.

```xml
SKV574008;
556000016701;2004;Per Persson;0123-45690; post@filmkopia.se
DE262875793;Y;;
```

### Exempel 3

Fil med ändring av redovisad avsedd köpare. Först anges VAT-nummer för den nya köparen. Därefter anges bokstaven Z som indikerar att uppgifterna avser byte av köpare och sedan anges den tidigare avsedda köparen. I exemplet nedan har köparen med VAT-nummer DE283993535 ersatt den tidigare avsedda köparen med VAT-nummer DE262875793.

```xml
SKV574008;
556000016701;2005;Per Persson;0123-45690; post@filmkopia.se
DE283993535;Z;DE262875793;
```