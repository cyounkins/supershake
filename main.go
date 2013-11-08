package main

import (
    "bufio"
    "encoding/csv"
    "fmt"
    "io"
    "math"
    "os"
    "regexp"
    "runtime/pprof"
    "strconv"
    "strings"
)

type Nutrient struct {
    id int
    units string
    description string
}

type NutrientInFood struct {
    nutrient Nutrient
    amountPerG float64
}

type Food struct {
    id int
    foodGroup string
    description string
    manufacturer string
    nutrients []NutrientInFood
}

func (food *Food) PrintNutrients(numGrams int) {
  for _, nutrientInFood := range food.nutrients {
    nutrient := nutrientInFood.nutrient
    totalUnits := nutrientInFood.amountPerG * float64(numGrams)
    if totalUnits >= 0.01 {
      fmt.Printf("%.2f%s of %s, ", totalUnits, nutrient.units, nutrient.description)
    }
  }
}

type Recipe struct {
    nutrientTotals map[int]float64 // nutrient id -> total quantity
    foodQuantities map[int]int // food id -> number of grams
}

func makeUSDADataReader(filename string) (*os.File, *csv.Reader) {
    inputFile, err := os.Open(filename)
    if err != nil { 
      fmt.Println("File not found. Download the USDA SR26 database from:")
      fmt.Println("https://www.ars.usda.gov/SP2UserFiles/Place/12354500/Data/SR26/dnload/sr26.zip")
      fmt.Println("Extract it and put this file next to the extracted files")
      panic(err) 
    }

    bufferedReader := bufio.NewReader(inputFile)

    csvReader := csv.NewReader(bufferedReader)
    csvReader.Comma = '^'
    csvReader.LazyQuotes = true
    csvReader.TrailingComma = true

    return inputFile, csvReader
}

func assertStringHasTwiddles(input string) {
    if input[0] != byte('~') || input[len(input) - 1] != byte('~') {
        panic("Expected twiddles in string: " + input)
    }
}

func stripTwiddles(input string) string {
    return input[1:len(input) - 1]
}

func getNutrientsAndFoods() (map[int]Nutrient, map[string]int, map[int]Food) {
    foodDescriptionFile, foodDescriptionReader := makeUSDADataReader("FOOD_DES.txt")
    nutrientDefinitionFile, nutrientDefinitionReader := makeUSDADataReader("NUTR_DEF.txt")
    foodDataFile, foodDataReader := makeUSDADataReader("NUT_DATA.txt")

    // close inputFile on exit and check for its returned error
    defer func() {
        if err := foodDescriptionFile.Close(); err != nil {
            panic(err)
        }
        if err := nutrientDefinitionFile.Close(); err != nil {
            panic(err)
        }
        if err := foodDataFile.Close(); err != nil {
            panic(err)
        }
    }()

    nutrients := make(map[int]Nutrient, 150)
    nutrientNameToId := make(map[string]int, 150)
    foods := make(map[int]Food, 5000)

    // Read from NUTR_DEF.txt
    for {
        record, err := nutrientDefinitionReader.Read()
        if err == io.EOF {
            break
        } else if err != nil {
            panic(err)
        }

        assertStringHasTwiddles(record[0])
        assertStringHasTwiddles(record[1])
        assertStringHasTwiddles(record[3])

        id, err := strconv.Atoi(stripTwiddles(record[0]))
        if err != nil { panic(err) }
        units := stripTwiddles(record[1])
        description := stripTwiddles(record[3])

        // Drop the \d:\d entries but keep three-letter abbreviated ones
        matched, err := regexp.MatchString("^\\d+:\\d+", description)
        if err != nil { panic(err) }
        if matched {
          matched, err := regexp.MatchString("\\(\\w{3}\\)", description)
          if err != nil { panic(err) }
          if !matched {
            continue
          }
        }

        // Correction of duplicate description field
        if id == 208 {
            description = "Energy, kcal"
        } else if id == 268 {
            description = "Energy, kJ"
        }

        _, exists := nutrients[id]
        if exists {
            panic("nutrient already in nutrients map")
        }

        n := Nutrient{}
        n.id = id
        n.units = units
        n.description = description
        //fmt.Printf("%s - %s\n", description, units)

        nutrients[id] = n

        nutrientNameToId[description] = id
    }

    // Read from FOOD_DES.txt
    for {
        record, err := foodDescriptionReader.Read()
        if err == io.EOF {
            break
        } else if err != nil {
            panic(err)
        }

        assertStringHasTwiddles(record[0])
        assertStringHasTwiddles(record[1])
        assertStringHasTwiddles(record[2])

        ndb, err := strconv.Atoi(stripTwiddles(record[0]))
        if err != nil { panic(err) }
        foodGroup := stripTwiddles(record[1])
        description := stripTwiddles(record[2])
        manufacturer := stripTwiddles(record[5])

        if foodGroup == "0300" || // baby foods
           foodGroup == "0800" || // breakfast cereals
           foodGroup == "1400" || // beverages
           foodGroup == "2100" || // fast foods
           foodGroup == "3600" { // restaurant foods
            continue
        }

        if strings.Contains(description, "Lemonade") ||
           strings.Contains(description, "Ice cream") ||
           strings.Contains(description, "dehydrated flakes") ||
           strings.Contains(description, "Alcoholic beverage") ||
           strings.Contains(description, "freeze-dried") ||
           strings.Contains(description, "Celery flakes") ||
           strings.Contains(description, "dehydrated") ||
           strings.Contains(description, "Candies") ||
           strings.Contains(description, "Tea,") ||
           //strings.Contains(strings.ToLower(description), " dried") ||

           // Meat
           strings.Contains(strings.ToLower(description), "beef,") || 
           strings.Contains(strings.ToLower(description), "pork,") || 
           strings.Contains(strings.ToLower(description), "pork skins,") || 
           strings.Contains(strings.ToLower(description), "chicken,") || 
           strings.Contains(strings.ToLower(description), "smelt,") || 
           strings.Contains(strings.ToLower(description), "salmon,") || 
           strings.Contains(strings.ToLower(description), "fish,") || 
           strings.Contains(strings.ToLower(description), "mutton,") || 
           strings.Contains(strings.ToLower(description), "turkey,") || 
           strings.Contains(strings.ToLower(description), "trout,") || 
           strings.Contains(strings.ToLower(description), "lamb,") || 
           strings.Contains(strings.ToLower(description), "caribou,") || 
           strings.Contains(strings.ToLower(description), " meat,") || 

           // manufactured, likely to contain additives
           strings.Contains(strings.ToLower(description), "liver cheese,") ||
           strings.Contains(description, "surimi") ||
           strings.Contains(strings.ToLower(description), "big franks,") || 
           strings.Contains(description, "MORNINGSTAR") ||
           strings.Contains(description, "Meat extender") ||
           strings.Contains(description, "with low-calorie sweeteners") ||
           strings.Contains(description, "instant breakfast powder") ||
           strings.Contains(description, "Orange-flavor drink") ||
           strings.Contains(description, "Fruit-flavored drink") ||
           strings.Contains(description, "Leavening agents") ||
           strings.Contains(description, "Reddi Wip") ||
           strings.Contains(description, "Frozen novelties") ||

           // added nutrients
           strings.Contains(description, "Formulated bar,") ||
           strings.Contains(strings.ToLower(description), " acid,") ||
           strings.Contains(strings.ToLower(description), " added ") ||
           strings.Contains(strings.ToLower(description), " supplement") ||
           strings.Contains(strings.ToLower(description), " fortified") ||
           strings.Contains(description, "Soy protein isolate") ||
           strings.Contains(description, "Soy protein concentrate") ||

           // hard to put in a shake
           //strings.Contains(description, " bran") ||
           //strings.Contains(description, " meal") ||
           //strings.Contains(description, " flour") ||
           //strings.Contains(description, "Wheat germ") ||
           strings.Contains(description, "PAM cooking spray") ||  // srsly wtf

           // animals
           strings.Contains(strings.ToLower(description), " seal,") ||
           strings.Contains(description, "Seal,") ||

           // access
           strings.Contains(description, "Egg Mix, USDA Commodity") ||
           strings.Contains(description, "Game meat") ||
           strings.Contains(description, "Butterbur, canned") ||

           // too expensive
           strings.Contains(strings.ToLower(description), "mollusks") ||
           strings.Contains(description, "Spices,") ||

           // body parts I probably won't eat
           strings.Contains(strings.ToLower(description), " brain") ||
           strings.Contains(strings.ToLower(description), " liver ") ||
           strings.Contains(strings.ToLower(description), " liver,") ||
           strings.Contains(strings.ToLower(description), " kidney") ||
           strings.Contains(strings.ToLower(description), " lungs,") ||

           // requires significant work to clean
           strings.Contains(strings.ToLower(description), " chitterlings") ||
           strings.Contains(strings.ToLower(description), " intestine") ||

           // High-mercury fish
           strings.Contains(strings.ToLower(description), " mackerel,") ||
           strings.Contains(strings.ToLower(description), " marlin,") ||
           strings.Contains(strings.ToLower(description), " orange roughy,") ||
           strings.Contains(strings.ToLower(description), " shark,") ||
           strings.Contains(strings.ToLower(description), " swordfish,") ||
           strings.Contains(strings.ToLower(description), " tilefish,") ||
           strings.Contains(strings.ToLower(description), " tuna,") ||
           strings.Contains(strings.ToLower(description), " bluefish,") ||
           strings.Contains(strings.ToLower(description), " grouper,") ||
           strings.Contains(strings.ToLower(description), " sea bass") ||
           strings.Contains(strings.ToLower(description), " bass,") ||
           strings.Contains(strings.ToLower(description), " carp,") ||
           strings.Contains(strings.ToLower(description), " cod,") ||
           strings.Contains(strings.ToLower(description), " croaker,") ||
           strings.Contains(strings.ToLower(description), " halibut,") ||
           strings.Contains(strings.ToLower(description), " jacksmelt,") ||
           strings.Contains(strings.ToLower(description), " lobster,") ||
           strings.Contains(strings.ToLower(description), " mahi mahi,") ||
           strings.Contains(strings.ToLower(description), " monkfish,") ||
           strings.Contains(strings.ToLower(description), " perch,") ||
           strings.Contains(strings.ToLower(description), " sablefish,") ||
           strings.Contains(strings.ToLower(description), " skate,") ||
           strings.Contains(strings.ToLower(description), " snapper,") ||
           strings.Contains(strings.ToLower(description), " weakfish,") || 
           strings.Contains(strings.ToLower(description), " whale,") {

            continue
        }

        if manufacturer == "Campbell Soup Co." {
            continue
        }

        _, exists := foods[ndb]
        if exists {
            panic("ndb already in foods map")
        }

        f := Food{}
        f.id = ndb
        f.foodGroup = foodGroup
        f.description = description
        f.manufacturer = manufacturer

        foods[ndb] = f
    }

    // Read from NUT_DATA.txt
    for {
        record, err := foodDataReader.Read()
        if err == io.EOF {
            break
        } else if err != nil {
            panic(err)
        }

        assertStringHasTwiddles(record[0])
        assertStringHasTwiddles(record[1])

        ndb, err := strconv.Atoi(stripTwiddles(record[0]))
        if err != nil { panic(err) }
        nutrientId, err := strconv.Atoi(stripTwiddles(record[1]))
        if err != nil { panic(err) }
        nutrientAmount64, err := strconv.ParseFloat(record[2], 64)
        if err != nil { panic(err) }
        numDataPoints, err := strconv.Atoi(record[3])
        if err != nil { panic(err) }

        // Including this because of the strangeness seen with heart of palm, raw
        // versus heart of palm, canned with respect to potassium (10x variance)
        // If the number of data points is 0, the value was calculated or imputed.
        if numDataPoints == 0 {
            // Assume they are wrong
            nutrientAmount64 = float64(0)
        }

        _, exists := nutrients[nutrientId]
        // Skip the nutrient if we skipped it on nutrient definition import
        if !exists {
          continue
        }

        nif := NutrientInFood{}
        nif.nutrient = nutrients[nutrientId]
        // divide by 100 because this measurement is for 100g
        nif.amountPerG = nutrientAmount64 / 100

        food, exists := foods[ndb]
        if !exists {
            continue
        }
        food.nutrients = append(food.nutrients, nif)
        foods[ndb] = food
    }

    return nutrients, nutrientNameToId, foods
}

func calcPenalty(nutrientName string, amount, min, max float64, verbose bool) float64 {
    if amount < min {
        penalty := (min - float64(amount))/min * float64(100)
        if verbose { fmt.Printf("Penalty for less %s than min (have %f, need %f): %f\n", nutrientName, amount, min, penalty) }
        return penalty
    } else {
        // amount >= min

        if max != 0 {
            minMaxMidpoint := min + (max - min) / 2

            if amount < minMaxMidpoint {
                // less than midpoint, no penalty
                if verbose { fmt.Printf("No penalty for %s\n", nutrientName) }
                return float64(0)
            } else {
                // linear penalty for above midpoint
                overBy := amount - minMaxMidpoint
                penalty := (overBy / (max - minMaxMidpoint)) * float64(100)
                if verbose { fmt.Printf("Penalty for excess %s (amount=%f, min=%f, max=%f): %f\n", nutrientName, amount, min, max, penalty)}
                return penalty
            }
        } else {
            if verbose { fmt.Printf("No penalty for %s\n", nutrientName) }
            return float64(0)
        }
    }
}

// ===========================================================================

func NewRecipe(allFoods map[int]Food, allNutrients map[int]Nutrient) *Recipe {
    recipe := Recipe{}
    recipe.nutrientTotals = make(map[int]float64, 150)
    recipe.foodQuantities = make(map[int]int, 50)

    for nutrientId := range allNutrients {
        recipe.nutrientTotals[nutrientId] = 0
    }

    recipe.AssertConsistency(allFoods)
    return &recipe
}

func (recipe1 *Recipe) Equals(recipe2 *Recipe, allFoods map[int]Food) bool {
    recipe1.AssertConsistency(allFoods)
    recipe2.AssertConsistency(allFoods)

    if len(recipe1.foodQuantities) != len(recipe2.foodQuantities) {
        return false
    }

    for key, value1 := range recipe1.foodQuantities {
        value2, exists := recipe2.foodQuantities[key]
        if !exists {
            return false
        }

        if value1 != value2 {
            return false
        }
    }

    return true
}

func (recipe *Recipe) HasFood(food *Food) bool {
    _, exists := recipe.foodQuantities[food.id]
    return exists
}

func (recipe *Recipe) AddFood(allFoods map[int]Food, food *Food, quantityToAdd int) {
    recipe.AssertConsistency(allFoods)
    originalQuantity, exists := recipe.foodQuantities[food.id]
    
    if exists {
        recipe.foodQuantities[food.id] = originalQuantity + quantityToAdd
    } else {
        recipe.foodQuantities[food.id] = quantityToAdd
    }

    // Maintain consistency by updating the nutrientTotals list
    for _, nutrientInFood := range food.nutrients {
        // this code assumes the key exists as set up in New
        nutrientId := nutrientInFood.nutrient.id
        amountAdded := nutrientInFood.amountPerG * float64(quantityToAdd)
        recipe.nutrientTotals[nutrientId] += amountAdded
    }
    recipe.AssertConsistency(allFoods)
}

func (recipe *Recipe) RemoveFood(allFoods map[int]Food, food *Food, quantityToRemove int) {
    recipe.AssertConsistency(allFoods)
    originalQuantity, exists := recipe.foodQuantities[food.id]
    if !exists {
        panic("Asked to remove food that is not in recipe")
    }

    if quantityToRemove > originalQuantity {
        panic("Asked to remove more food than is in recipe")
    }

    if quantityToRemove == originalQuantity {
        delete(recipe.foodQuantities, food.id)
    } else {
        newQuantity := originalQuantity - quantityToRemove
        recipe.foodQuantities[food.id] = newQuantity
    }

    // Maintain consistency by updating the nutrientTotals list
    for _, nutrientInFood := range food.nutrients {
        // this code assumes the key exists as set up in New
        nutrientId := nutrientInFood.nutrient.id
        amountRemoved := nutrientInFood.amountPerG * float64(quantityToRemove)
        recipe.nutrientTotals[nutrientId] -= amountRemoved
    }

    recipe.AssertConsistency(allFoods)
}

func (recipe *Recipe) AssertConsistency(allFoods map[int]Food) {
    // Ensure there are no 0 quantity foods
    /*for foodId, quantity := range recipe.foodQuantities {
        if quantity <= 0 {
            panic("Quantity <= 0 for food:" + string(foodId))
        }
    }

    // Separately sum up the nutrient totals
    nutrientTotals := make(map[int]float64)
    for foodId, quantity := range recipe.foodQuantities {
        food := allFoods[foodId]
        for _, nutrientInFood := range food.nutrients {
            nutrient := nutrientInFood.nutrient
            originalQuantity, exists := nutrientTotals[nutrient.id]
            if exists {
                nutrientTotals[nutrient.id] = originalQuantity + (nutrientInFood.amountPerG * float64(quantity))
            } else {
                nutrientTotals[nutrient.id] = nutrientInFood.amountPerG * float64(quantity)
            }
        }
    }

    // compare the separately computed nutrient totals with what is in the recipe
    for nutrientId, total := range nutrientTotals {
        if math.Abs(recipe.nutrientTotals[nutrientId] - total) > 0.5 {
            panic("Nutrient totals are not consistent.")
        }
    }*/
}

func (recipe *Recipe) Clone(allFoods map[int]Food, allNutrients map[int]Nutrient) *Recipe {
    recipe.AssertConsistency(allFoods)
    newRecipe := NewRecipe(allFoods, allNutrients)

    // Copy over food quantities
    for foodId, quantity := range recipe.foodQuantities {
        newRecipe.foodQuantities[foodId] = quantity
    }

    // Copy over nutrient totals
    for nutrientId, total := range recipe.nutrientTotals {
        newRecipe.nutrientTotals[nutrientId] = total
    }

    newRecipe.AssertConsistency(allFoods)
    return newRecipe
}

func (recipe *Recipe) calculatePenaltyForNutrient(nutrientNameToId map[string]int, nutrientName string, 
        min, max float64, verbose bool) float64 {

    nutrientId := nutrientNameToId[nutrientName]
    amount := recipe.nutrientTotals[nutrientId]
    return calcPenalty(nutrientName, amount, min, max, verbose)
}


func (recipe *Recipe) Score(nutrients map[int]Nutrient, allFoods map[int]Food, nutrientNameToId map[string]int, verbose bool) float64 {
    // For each nutrient, assign a penalty of up to 100, scaled by
    // amount of nutrient that is missing.
    // That is, 100 = none of the nutrient, 0 = suffient amount
    // Assign 100 if nutrient is above recommended intake

    // 145 lbs = 65kg

    // Not reported nutrients
    // Biotin
    // Chloride
    // Chromium
    // Iodine - 150ug <= Iodine <= 1100ug
    // Molybdenum <= 10mg

    // Reported nutrients not used

    // Alanine - nonessential amino acid
    // Arginine - nonessential amino acid
    // Aspartic acid - nonessential amino acid
    // Beta-sitosterol - phytosterol
    // Betaine
    // Campesterol - phytosterol
    // Carotene, beta
    // Carotene, alpha
    // Cholesterol
    // Cryptoxanthin, beta
    // Fatty acids
    // Fluoride
    // Folic acid - covered by Folate, DFE
    // Fructose
    // Galactose
    // Glucose (dextrose)
    // Glutamic acid - nonessential amino acid
    // Glycine - nonessential amino acid
    // Hydroxyproline
    // Lactose
    // Lycopene
    // Menaquinone-4
    // Phytosterols
    // Proline - nonessential amino acid
    // Retinol
    // Serine - nonessential amino acid
    // Starch
    // Stigmasterol - phytosterol
    // Sucrose
    // Sugars, total
    // Theobromine
    // Tocopherol, beta
    // Tocopherol, delta
    // Tocopherol, gamma
    // Tocotrienol, alpha
    // Tocotrienol, beta
    // Tocotrienol, delta
    // Tocotrienol, gamma
    // Total lipid (fat)
    // Vitamin D (D2 + D3)
    // Vitamin D2 (ergocalciferol)
    // Vitamin D3 (cholecalciferol)
    // Water
    // Omega-6 (18:3 n-6 c,c,c)

    recipe.AssertConsistency(allFoods)
    penalty := float64(0)

    // Need some fat, and not too concerned about excess intake given my build,
    // but let's not go crazy with it.
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Total lipid (fat)", 60, 300, verbose)

    // 2700 kcal recommended for men
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Energy, kcal", 2700, 10000, verbose)

    // 51g <= protein <= 3510g (?!)
    // 51g is recommended minimum
    // 0.82 g/lb is the upper limit of useful protein intake
    // http://mennohenselmans.com/the-myth-of-1glb-optimal-protein-intake-for-bodybuilders/
    // 145 * 0.7 = 101.5
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Protein", 101.5, 3510, verbose)

    // 38g <= Fiber, total dietary
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Fiber, total dietary", 38, 0, verbose)

    // 1000mg <= Calcium, Ca <= 2500mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Calcium, Ca", 1000, 2500, verbose)

    // 8mg <= Iron, Fe <= 45mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Iron, Fe", 8, 45, verbose)

    // 400mg <= Magnesium, Mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Magnesium, Mg", 400, 0, verbose)

    // 700mg <= Phosphorus, P <= 4000mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Phosphorus, P", 700, 4000, verbose)

    // 4700mg <= Potassium, K
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Potassium, K", 4700, 0, verbose)

    // 1500mg <= Sodium, Na <= 2300mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Sodium, Na", 1500, 2300, verbose)

    // 11mg <= Zinc, Zn <= 40mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Zinc, Zn", 11, 40, verbose)

    // 0.9mg <= Copper, Cu <= 10mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Copper, Cu", 0.9, 10, verbose)

    // 2.3mg <= Manganese, Mn <= 11mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Manganese, Mn", 2.3, 11, verbose)

    // 55ug <= Selenium, Se <= 400ug
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Selenium, Se", 55, 400, verbose)

    // 900ug <= Vitamin A, RAE <= 1500ug
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Vitamin A, RAE", 900, 1500, verbose)

    // 15mg <= Vitamin E (alpha-tocopherol) <= 1000mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Vitamin E (alpha-tocopherol)", 15, 1000, verbose)

    // 10000ug <= Lutein and 2000ug <= zeaxanthin OR 12000ug <= Lutein + zeaxanthin
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Lutein + zeaxanthin", 12000, 0, verbose)

    // 90mg <= Vitamin C, total ascorbic acid <= 2000mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Vitamin C, total ascorbic acid", 90, 2000, verbose)

    // 1.2mg <= Thiamin
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Thiamin", 1.2, 0, verbose)

    // 1.3mg <= Riboflavin
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Riboflavin", 1.3, 0, verbose)

    // 16mg <= Niacin <= 35mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Niacin", 16, 35, verbose)

    // 5mg <= Pantothenic acid
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Pantothenic acid", 5, 0, verbose)

    // 1.3mg <= Vitamin B-6 <= 100mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Vitamin B-6", 1.3, 100, verbose)

    // 2.4ug <= Vitamin B-12
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Vitamin B-12", 2.4, 0, verbose)

    // 550mg <= Choline, total <= 3500mg
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Choline, total", 550, 3500, verbose)

    // 120ug <= Vitamin K (phylloquinone)
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Vitamin K (phylloquinone)", 120, 0, verbose)

    // 1.95g <= Lysine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Lysine", 1.95, 0, verbose)

    // 2.535g <= Leucine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Leucine", 2.535, 0, verbose)

    // 0.65g <= Methionine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Methionine", 0.65, 0, verbose)

    // 0.26g <= Cystine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Cystine", 0.26, 0, verbose)

    // 1.69g <= Valine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Valine", 1.69, 0, verbose)

    // 0.65g <= Histidine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Histidine", 0.65, 0, verbose)

    // 0.26g <= Tryptophan
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Tryptophan", 0.26, 0, verbose)

    // 0.975g <= Threonine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Threonine", 0.975, 0, verbose)

    // 1.3g <= Isoleucine
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Isoleucine", 1.3, 0, verbose)

    // 1.6g <= 18:3 n-3 c,c,c (ALA)   // Omega-3
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "18:3 n-3 c,c,c (ALA)", 1.6, 0, verbose)

    // 1.6g <= 20:5 n-3 (EPA)      // Omega-3
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "20:5 n-3 (EPA)", 1.6, 0, verbose)

    // 1.6g <= 22:6 n-3 (DHA)      // Omega-3
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "22:6 n-3 (DHA)", 1.6, 0, verbose)

    // half water from food
    // 64 fl oz recommended daily
    // 32 fl oz = 946 grams
    penalty += recipe.calculatePenaltyForNutrient(nutrientNameToId, "Water", 946, 0, verbose)

    // 1.625g <= Phenylalanine + Tyrosine
    amountPhenylalanine, exists := recipe.nutrientTotals[nutrientNameToId["Phenylalanine"]]
    if !exists { amountPhenylalanine = 0 }
    amountTyrosine, exists := recipe.nutrientTotals[nutrientNameToId["Tyrosine"]]
    if !exists { amountTyrosine = 0 }
    pt := amountPhenylalanine + amountTyrosine
    penalty += calcPenalty("Phenylalanine + Tyrosine", pt, 1.625, 0, verbose)

    // Folate DFE
    // 400 <= Folate, DFE <= 1000
    foodFolate := recipe.nutrientTotals[nutrientNameToId["Folate, food"]]
    folicAcid := recipe.nutrientTotals[nutrientNameToId["Folic acid"]]
    folateDFE := foodFolate + (1.7 * folicAcid)
    penalty += calcPenalty("Folate", folateDFE, 400, 1000, verbose)

    // Caffeine should be reduced
    if recipe.nutrientTotals[nutrientNameToId["Caffeine"]] > 20 {
        caffeinePenalty := (recipe.nutrientTotals[nutrientNameToId["Caffeine"]] - 5)
        if verbose { fmt.Printf("Penalty for caffeine: %f\n", caffeinePenalty) }
        penalty += caffeinePenalty
    }

    // Dihydrophylloquinone is linked to low bone density
    penalty += recipe.nutrientTotals[nutrientNameToId["Dihydrophylloquinone"]]

    // Penalize by number of non-zero components
    numFoods := 0
    for _, grams := range recipe.foodQuantities {
        if grams != 0 {
            numFoods += 1
        }
    }
    numFoodsPenalty := math.Min(float64(numFoods) / 100, 1) * 10
    if verbose { fmt.Printf("Penalty for num foods: %f\n", numFoodsPenalty) }
    penalty += numFoodsPenalty

    // Penalize more matter
    totalMass := int(0)
    for _, grams := range recipe.foodQuantities {
        totalMass += grams
    }
    massPenalty := math.Min(float64(totalMass) / 3000, 1) * 10
    if verbose { fmt.Printf("Penalty for mass: %f\n", massPenalty) }
    penalty += massPenalty

    return penalty
}

func (recipe *Recipe) PrintTotalNutrients(allNutrients map[int]Nutrient) {
  for nutrientId, amount := range recipe.nutrientTotals {
    nutrient := allNutrients[nutrientId]
    fmt.Printf("%.2f%s of %s\n", amount, nutrient.units, nutrient.description)
  }
}

// ===========================================================================

func main () {
    fmt.Println("Loading")
    STEPSIZE := int(5)

    f, err := os.Create("cpuProfile")
    if err != nil {
        panic(err)
    }
    pprof.StartCPUProfile(f)
    defer pprof.StopCPUProfile()

    allNutrients, nutrientNameToId, allFoods := getNutrientsAndFoods()

    bestRecipeEver := NewRecipe(allFoods, allNutrients)
    bestScoreEver := bestRecipeEver.Score(allNutrients, allFoods, nutrientNameToId, false)

    for bestScoreEver > 0 {
        fmt.Println(bestRecipeEver.foodQuantities)
        fmt.Println("Best score ever", bestScoreEver)

        var bestRecipeThisRound *Recipe
        bestScoreThisRound := bestScoreEver 

        // Start from the best ever
        // This one moves around the search space, testing the options
        // it must be cloned into bestRecipeThisRound!
        currentRecipe := bestRecipeEver.Clone(allFoods, allNutrients)    

        for _, food := range allFoods {
            var newScore float64

            /*if !currentRecipe.Equals(bestRecipeEver, allFoods) {
                fmt.Println(bestRecipeEver)
                fmt.Println(currentRecipe)
                panic("did not undo all steps")
            }*/

            // try removing 
            if currentRecipe.HasFood(&food) {
                currentRecipe.RemoveFood(allFoods, &food, STEPSIZE)
                newScore = currentRecipe.Score(allNutrients, allFoods, nutrientNameToId, false)
                if newScore < bestScoreThisRound {
                    // Better, woo!
                    bestRecipeThisRound = currentRecipe.Clone(allFoods, allNutrients)
                    bestScoreThisRound = newScore
                }
                // always undo
                currentRecipe.AddFood(allFoods, &food, STEPSIZE)
            }

            // =================================

            // try adding 
            currentRecipe.AddFood(allFoods, &food, STEPSIZE)
            newScore = currentRecipe.Score(allNutrients, allFoods, nutrientNameToId, false)
            if newScore < bestScoreThisRound {
                // Better, woo!
                bestRecipeThisRound = currentRecipe.Clone(allFoods, allNutrients)
                bestScoreThisRound = newScore
            }
            // always undo
            currentRecipe.RemoveFood(allFoods, &food, STEPSIZE)
        }

        if bestRecipeThisRound == nil {
            // We never got a chance to set bestRecipeThisRound,
            // which means we found nothing better than bestRecipeEver

            fmt.Println("Reached local maxima")
            fmt.Println(bestRecipeEver)
            bestRecipeEver.Score(allNutrients, allFoods, nutrientNameToId, true)
            for foodId, grams := range bestRecipeEver.foodQuantities {
                food := allFoods[foodId]
                fmt.Printf("%d grams of %s\n", grams, food.description)
                food.PrintNutrients(grams)
                fmt.Println("\n")
            }
            fmt.Println("TOTAL NUTRIENTS")
            bestRecipeEver.PrintTotalNutrients(allNutrients)
            break
        } else {
            if bestScoreThisRound > bestScoreEver {
                panic("wtf")
            }
            // Done trying all the foods
            bestRecipeEver = bestRecipeThisRound
            bestScoreEver = bestScoreThisRound
        }
    }
}


